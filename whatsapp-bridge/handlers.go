package main

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
)

// Extract text content from a message
func extractTextContent(msg *waProto.Message) string {
	if msg == nil {
		return ""
	}

	// Try to get text content
	if text := msg.GetConversation(); text != "" {
		return text
	} else if extendedText := msg.GetExtendedTextMessage(); extendedText != nil {
		return extendedText.GetText()
	}

	// For now, we're ignoring non-text messages
	return ""
}

// Handle regular incoming messages with media support
func handleMessage(rt *tenantRuntime, msg *events.Message, logger waLog.Logger) {
	rt.mu.RLock()
	client := rt.client
	messageStore := rt.store
	rt.mu.RUnlock()
	if client == nil || messageStore == nil {
		logger.Warnf("Skipping message for a runtime without client or store")
		return
	}

	// Store direct chats and senders under their canonical phone identity.
	chat := messageStore.CanonicalJID(client, msg.Info.Chat)
	senderJID := messageStore.CanonicalJID(client, msg.Info.Sender)
	chatJID := chat.String()
	sender := senderJID.User

	// Get appropriate chat name (pass nil for conversation since we don't have one for regular messages)
	name := GetChatName(client, messageStore, chat, chatJID, nil, sender, logger)

	// Update chat in database with the message timestamp (keeps last message time updated)
	err := messageStore.StoreChat(chatJID, name, msg.Info.Timestamp)
	if err != nil {
		logger.Warnf("Failed to store chat: %v", err)
	}

	// Extract text content
	content := extractTextContent(msg.Message)

	// Extract media info
	mediaType, filename, url, mediaKey, fileSHA256, fileEncSHA256, fileLength := extractMediaInfo(msg.Message)

	// Skip if there's no content and no media
	if content == "" && mediaType == "" {
		return
	}

	isGroup := chat.Server == types.GroupServer
	mentionedMe, replyToMe := messageContext(client, msg.Message)
	meta := &MessageEventMeta{
		ChatName:    name,
		SenderJID:   senderJID.String(),
		SenderName:  messageStore.ContactName(senderJID),
		IsGroup:     isGroup,
		MentionedMe: mentionedMe,
		IsReplyToMe: replyToMe,
	}

	// Store message and its event atomically
	_, err = messageStore.StoreMessage(StoredMessage{
		ID:            msg.Info.ID,
		ChatJID:       chatJID,
		Sender:        sender,
		Content:       content,
		Timestamp:     msg.Info.Timestamp,
		IsFromMe:      msg.Info.IsFromMe,
		MediaType:     mediaType,
		Filename:      filename,
		URL:           url,
		MediaKey:      mediaKey,
		FileSHA256:    fileSHA256,
		FileEncSHA256: fileEncSHA256,
		FileLength:    fileLength,
	}, meta)

	if err != nil {
		logger.Warnf("Failed to store message: %v", err)
	} else {
		// Log message reception
		timestamp := msg.Info.Timestamp.Format("2006-01-02 15:04:05")
		direction := "←"
		if msg.Info.IsFromMe {
			direction = "→"
		}

		// Log based on message type
		if mediaType != "" {
			fmt.Printf("[%s] %s %s: [%s: %s] %s\n", timestamp, direction, sender, mediaType, filename, content)
		} else if content != "" {
			fmt.Printf("[%s] %s %s: %s\n", timestamp, direction, sender, content)
		}
	}
}

// messageContext reports whether the message mentions this account or quotes
// one of its messages, by phone number or privacy LID.
func messageContext(client *whatsmeow.Client, msg *waProto.Message) (bool, bool) {
	if client == nil || client.Store == nil || msg == nil {
		return false, false
	}

	var contextInfo *waProto.ContextInfo
	switch {
	case msg.GetExtendedTextMessage() != nil:
		contextInfo = msg.GetExtendedTextMessage().GetContextInfo()
	case msg.GetImageMessage() != nil:
		contextInfo = msg.GetImageMessage().GetContextInfo()
	case msg.GetVideoMessage() != nil:
		contextInfo = msg.GetVideoMessage().GetContextInfo()
	case msg.GetDocumentMessage() != nil:
		contextInfo = msg.GetDocumentMessage().GetContextInfo()
	case msg.GetAudioMessage() != nil:
		contextInfo = msg.GetAudioMessage().GetContextInfo()
	}
	if contextInfo == nil {
		return false, false
	}

	var self []string
	if client.Store.ID != nil {
		self = append(self, client.Store.ID.ToNonAD().String())
		if client.Store.LIDs != nil {
			if lid, err := client.Store.LIDs.GetLIDForPN(context.Background(), client.Store.ID.ToNonAD()); err == nil && !lid.IsEmpty() {
				self = append(self, lid.String())
			}
		}
	}
	if !client.Store.LID.IsEmpty() {
		self = append(self, client.Store.LID.String())
	}

	mentioned := false
	for _, mentionedJID := range contextInfo.GetMentionedJID() {
		for _, own := range self {
			if jidMatches(mentionedJID, own) {
				mentioned = true
				break
			}
		}
	}

	replyToMe := false
	if participant := contextInfo.GetParticipant(); participant != "" {
		for _, own := range self {
			if jidMatches(participant, own) {
				replyToMe = true
				break
			}
		}
	}

	return mentioned, replyToMe
}

// GetChatName determines the appropriate name for a chat based on JID and other info
func GetChatName(client *whatsmeow.Client, messageStore *MessageStore, jid types.JID, chatJID string, conversation interface{}, sender string, logger waLog.Logger) string {
	// First, check if chat already exists in database with a name
	var existingName string
	err := messageStore.db.QueryRow("SELECT name FROM chats WHERE jid = ?", chatJID).Scan(&existingName)
	if err == nil && existingName != "" {
		// Chat exists with a name, use that
		logger.Infof("Using existing chat name for %s: %s", chatJID, existingName)
		return existingName
	}

	// Need to determine chat name
	var name string

	if jid.Server == "g.us" {
		// This is a group chat
		logger.Infof("Getting name for group: %s", chatJID)

		// Use conversation data if provided (from history sync)
		if conversation != nil {
			// Extract name from conversation if available
			// This uses type assertions to handle different possible types
			var displayName, convName *string
			// Try to extract the fields we care about regardless of the exact type
			v := reflect.ValueOf(conversation)
			if v.Kind() == reflect.Ptr && !v.IsNil() {
				v = v.Elem()

				// Try to find DisplayName field
				if displayNameField := v.FieldByName("DisplayName"); displayNameField.IsValid() && displayNameField.Kind() == reflect.Ptr && !displayNameField.IsNil() {
					dn := displayNameField.Elem().String()
					displayName = &dn
				}

				// Try to find Name field
				if nameField := v.FieldByName("Name"); nameField.IsValid() && nameField.Kind() == reflect.Ptr && !nameField.IsNil() {
					n := nameField.Elem().String()
					convName = &n
				}
			}

			// Use the name we found
			if displayName != nil && *displayName != "" {
				name = *displayName
			} else if convName != nil && *convName != "" {
				name = *convName
			}
		}

		// If we didn't get a name, try group info
		if name == "" {
			groupInfo, err := client.GetGroupInfo(context.Background(), jid)
			if err == nil && groupInfo.Name != "" {
				name = groupInfo.Name
			} else {
				// Fallback name for groups
				name = fmt.Sprintf("Group %s", jid.User)
			}
		}

		logger.Infof("Using group name: %s", name)
	} else {
		// This is an individual contact
		logger.Infof("Getting name for contact: %s", chatJID)

		name = messageStore.ContactName(jid)
		contact, err := client.Store.Contacts.GetContact(context.Background(), jid)
		if name == "" && err == nil && contact.FullName != "" {
			name = contact.FullName
		}
		if name == "" && sender != "" {
			name = sender
		}
		if name == "" {
			name = jid.User
		}

		logger.Infof("Using contact name: %s", name)
	}

	return name
}

// Handle history sync events
func handleHistorySync(rt *tenantRuntime, historySync *events.HistorySync, logger waLog.Logger) {
	rt.mu.RLock()
	client := rt.client
	messageStore := rt.store
	rt.mu.RUnlock()
	if client == nil || messageStore == nil {
		logger.Warnf("Skipping history sync for a runtime without client or store")
		return
	}

	fmt.Printf("Received history sync event with %d conversations\n", len(historySync.Data.Conversations))

	syncedCount := 0
	for _, conversation := range historySync.Data.Conversations {
		// Parse JID from the conversation
		if conversation.ID == nil {
			continue
		}

		originalChatJID := *conversation.ID

		// Try to parse and canonicalize the JID
		jid, err := types.ParseJID(originalChatJID)
		if err != nil {
			logger.Warnf("Failed to parse JID %s: %v", originalChatJID, err)
			continue
		}
		jid = messageStore.CanonicalJID(client, jid)
		chatJID := jid.String()

		// Get appropriate chat name by passing the history sync conversation directly
		name := GetChatName(client, messageStore, jid, chatJID, conversation, "", logger)

		// Process messages
		messages := conversation.Messages
		if len(messages) > 0 {
			// Update chat with latest message timestamp
			latestMsg := messages[0]
			if latestMsg == nil || latestMsg.Message == nil {
				continue
			}

			// Get timestamp from message info
			timestamp := time.Time{}
			if ts := latestMsg.Message.GetMessageTimestamp(); ts != 0 {
				timestamp = time.Unix(int64(ts), 0)
			} else {
				continue
			}

			messageStore.StoreChat(chatJID, name, timestamp)

			// Store messages
			for _, msg := range messages {
				if msg == nil || msg.Message == nil {
					continue
				}

				// Extract text content
				var content string
				if msg.Message.Message != nil {
					if conv := msg.Message.Message.GetConversation(); conv != "" {
						content = conv
					} else if ext := msg.Message.Message.GetExtendedTextMessage(); ext != nil {
						content = ext.GetText()
					}
				}

				// Extract media info
				var mediaType, filename, url string
				var mediaKey, fileSHA256, fileEncSHA256 []byte
				var fileLength uint64

				if msg.Message.Message != nil {
					mediaType, filename, url, mediaKey, fileSHA256, fileEncSHA256, fileLength = extractMediaInfo(msg.Message.Message)
				}

				// Log the message content for debugging
				logger.Infof("Message content: %v, Media Type: %v", content, mediaType)

				// Skip messages with no content and no media
				if content == "" && mediaType == "" {
					continue
				}

				// Determine sender
				var sender string
				isFromMe := false
				if msg.Message.Key != nil {
					if msg.Message.Key.FromMe != nil {
						isFromMe = *msg.Message.Key.FromMe
					}
					if !isFromMe && msg.Message.Key.Participant != nil && *msg.Message.Key.Participant != "" {
						sender = *msg.Message.Key.Participant
					} else if isFromMe {
						sender = client.Store.ID.User
					} else {
						sender = jid.User
					}
				} else {
					sender = jid.User
				}

				if senderJID, parseErr := types.ParseJID(sender); parseErr == nil {
					sender = messageStore.CanonicalJID(client, senderJID).User
				}

				// Store message
				msgID := ""
				if msg.Message.Key != nil && msg.Message.Key.ID != nil {
					msgID = *msg.Message.Key.ID
				}

				// Get message timestamp
				timestamp := time.Time{}
				if ts := msg.Message.GetMessageTimestamp(); ts != 0 {
					timestamp = time.Unix(int64(ts), 0)
				} else {
					continue
				}

				_, err = messageStore.StoreMessage(StoredMessage{
					ID:            msgID,
					ChatJID:       chatJID,
					Sender:        sender,
					Content:       content,
					Timestamp:     timestamp,
					IsFromMe:      isFromMe,
					MediaType:     mediaType,
					Filename:      filename,
					URL:           url,
					MediaKey:      mediaKey,
					FileSHA256:    fileSHA256,
					FileEncSHA256: fileEncSHA256,
					FileLength:    fileLength,
				}, nil)
				if err != nil {
					logger.Warnf("Failed to store history message: %v", err)
				} else {
					syncedCount++
					// Log successful message storage
					if mediaType != "" {
						logger.Infof("Stored message: [%s] %s -> %s: [%s: %s] %s",
							timestamp.Format("2006-01-02 15:04:05"), sender, chatJID, mediaType, filename, content)
					} else {
						logger.Infof("Stored message: [%s] %s -> %s: %s",
							timestamp.Format("2006-01-02 15:04:05"), sender, chatJID, content)
					}
				}
			}
		}
	}

	fmt.Printf("History sync complete. Stored %d messages.\n", syncedCount)
}

// Request history sync from the server
func requestHistorySync(client *whatsmeow.Client) {
	if client == nil {
		fmt.Println("Client is not initialized. Cannot request history sync.")
		return
	}

	if !client.IsConnected() {
		fmt.Println("Client is not connected. Please ensure you are connected to WhatsApp first.")
		return
	}

	if client.Store.ID == nil {
		fmt.Println("Client is not logged in. Please scan the QR code first.")
		return
	}

	// Build and send a history sync request
	historyMsg := client.BuildHistorySyncRequest(nil, 100)
	if historyMsg == nil {
		fmt.Println("Failed to build history sync request.")
		return
	}

	_, err := client.SendMessage(context.Background(), types.JID{
		Server: "s.whatsapp.net",
		User:   "status",
	}, historyMsg)

	if err != nil {
		fmt.Printf("Failed to request history sync: %v\n", err)
	} else {
		fmt.Println("History sync requested. Waiting for server response...")
	}
}
