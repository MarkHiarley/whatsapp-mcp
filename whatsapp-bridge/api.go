package main

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net/http"

	"go.mau.fi/whatsmeow"
)

// SendMessageResponse is kept for the deprecated unversioned endpoint.
type SendMessageResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type APIResponse struct {
	Success bool      `json:"success"`
	Data    any       `json:"data"`
	Error   *APIError `json:"error"`
}

type SendMessageData struct {
	Message string `json:"message"`
}

// SendMessageRequest represents the request body for the send message API
type SendMessageRequest struct {
	Recipient string `json:"recipient"`
	Message   string `json:"message"`
	MediaPath string `json:"media_path,omitempty"`
}

// DownloadMediaRequest represents the request body for the download media API
type DownloadMediaRequest struct {
	MessageID string `json:"message_id"`
	ChatJID   string `json:"chat_jid"`
}

// DownloadMediaResponse is kept for the deprecated unversioned endpoint.
type DownloadMediaResponse struct {
	Success  bool   `json:"success"`
	Message  string `json:"message"`
	Filename string `json:"filename,omitempty"`
	Path     string `json:"path,omitempty"`
}

type DownloadMediaData struct {
	Message  string `json:"message"`
	Filename string `json:"filename"`
	Path     string `json:"path"`
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeAPIError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, APIResponse{
		Success: false,
		Data:    nil,
		Error:   &APIError{Code: code, Message: message},
	})
}

func requireBearerToken(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expected := "Bearer " + token
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte(expected)) != 1 {
			writeAPIError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid or missing bearer token")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func contractError(w http.ResponseWriter, status int, code, message string, legacy bool) {
	if legacy {
		http.Error(w, message, status)
		return
	}
	writeAPIError(w, status, code, message)
}

func newRESTHandler(client *whatsmeow.Client, messageStore *MessageStore, token string) http.Handler {
	mux := http.NewServeMux()

	sendHandler := func(legacy bool) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if legacy {
				w.Header().Set("Deprecation", "true")
			}
			if r.Method != http.MethodPost {
				contractError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed", legacy)
				return
			}

			var req SendMessageRequest
			decoder := json.NewDecoder(r.Body)
			if !legacy {
				decoder.DisallowUnknownFields()
			}
			if err := decoder.Decode(&req); err != nil {
				contractError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request format", legacy)
				return
			}
			if req.Recipient == "" {
				contractError(w, http.StatusBadRequest, "INVALID_REQUEST", "Recipient is required", legacy)
				return
			}
			if req.Message == "" && req.MediaPath == "" {
				contractError(w, http.StatusBadRequest, "INVALID_REQUEST", "Message or media path is required", legacy)
				return
			}

			success, message := sendWhatsAppMessage(client, req.Recipient, req.Message, req.MediaPath)
			if legacy {
				status := http.StatusOK
				if !success {
					status = http.StatusInternalServerError
				}
				writeJSON(w, status, SendMessageResponse{Success: success, Message: message})
				return
			}
			if !success {
				writeAPIError(w, http.StatusInternalServerError, "SEND_FAILED", message)
				return
			}
			writeJSON(w, http.StatusOK, APIResponse{
				Success: true,
				Data:    SendMessageData{Message: message},
				Error:   nil,
			})
		}
	}

	downloadHandler := func(legacy bool) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if legacy {
				w.Header().Set("Deprecation", "true")
			}
			if r.Method != http.MethodPost {
				contractError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed", legacy)
				return
			}

			var req DownloadMediaRequest
			decoder := json.NewDecoder(r.Body)
			if !legacy {
				decoder.DisallowUnknownFields()
			}
			if err := decoder.Decode(&req); err != nil {
				contractError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request format", legacy)
				return
			}
			if req.MessageID == "" || req.ChatJID == "" {
				contractError(w, http.StatusBadRequest, "INVALID_REQUEST", "Message ID and chat JID are required", legacy)
				return
			}

			success, mediaType, filename, path, err := downloadMedia(client, messageStore, req.MessageID, req.ChatJID)
			if !success || err != nil {
				message := "Failed to download media"
				if err != nil {
					message += ": " + err.Error()
				}
				if legacy {
					writeJSON(w, http.StatusInternalServerError, DownloadMediaResponse{Success: false, Message: message})
				} else {
					writeAPIError(w, http.StatusInternalServerError, "DOWNLOAD_FAILED", message)
				}
				return
			}

			message := fmt.Sprintf("Successfully downloaded %s media", mediaType)
			if legacy {
				writeJSON(w, http.StatusOK, DownloadMediaResponse{
					Success: true, Message: message, Filename: filename, Path: path,
				})
				return
			}
			writeJSON(w, http.StatusOK, APIResponse{
				Success: true,
				Data: DownloadMediaData{
					Message: message, Filename: filename, Path: path,
				},
				Error: nil,
			})
		}
	}

	mux.HandleFunc("/api/v1/send", sendHandler(false))
	mux.HandleFunc("/api/v1/download", downloadHandler(false))
	mux.HandleFunc("/api/send", sendHandler(true))
	mux.HandleFunc("/api/download", downloadHandler(true))
	return requireBearerToken(token, mux)
}

// Start a REST API server to expose the WhatsApp client functionality.
func startRESTServer(client *whatsmeow.Client, messageStore *MessageStore, port int, token string) {
	serverAddr := fmt.Sprintf(":%d", port)
	fmt.Printf("Starting REST API server on %s...\n", serverAddr)
	go func() {
		if err := http.ListenAndServe(serverAddr, newRESTHandler(client, messageStore, token)); err != nil {
			fmt.Printf("REST API server error: %v\n", err)
		}
	}()
}
