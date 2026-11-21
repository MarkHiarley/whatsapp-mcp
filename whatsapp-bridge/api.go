package main

import (
	"encoding/json"
	"net/http"
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

func contractError(w http.ResponseWriter, status int, code, message string, legacy bool) {
	if legacy {
		http.Error(w, message, status)
		return
	}
	writeAPIError(w, status, code, message)
}
