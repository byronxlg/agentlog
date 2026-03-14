package daemon

import "encoding/json"

// Request represents a JSON-RPC-style request from a client to the daemon.
type Request struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

// Response represents the daemon's reply to a client request.
type Response struct {
	OK     bool            `json:"ok"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  string          `json:"error,omitempty"`
}

// WriteParams holds the parameters for a "write" request.
type WriteParams struct {
	Entry EntryParams `json:"entry"`
}

// EntryParams holds the fields for an entry to be written.
type EntryParams struct {
	ID        string   `json:"id,omitempty"`
	Timestamp string   `json:"timestamp,omitempty"`
	SessionID string   `json:"session_id"`
	Type      string   `json:"type"`
	Title     string   `json:"title"`
	Body      string   `json:"body,omitempty"`
	Tags      []string `json:"tags,omitempty"`
	FileRefs  []string `json:"file_refs,omitempty"`
}

// QueryParams holds the filter parameters for a "query" request.
type QueryParams struct {
	Type      string   `json:"type,omitempty"`
	SessionID string   `json:"session_id,omitempty"`
	Tags      []string `json:"tags,omitempty"`
	FilePath  string   `json:"file_path,omitempty"`
	Start     string   `json:"start,omitempty"`
	End       string   `json:"end,omitempty"`
}

// SearchParams holds the parameters for a "search" request.
type SearchParams struct {
	Query string `json:"query"`
}

// GetSessionParams holds the parameters for a "get_session" request.
type GetSessionParams struct {
	SessionID string `json:"session_id"`
}

// BlameParams holds the parameters for a "blame" request.
type BlameParams struct {
	FilePath string `json:"file_path"`
}

func okResponse(result any) Response {
	data, err := json.Marshal(result)
	if err != nil {
		return errResponse("marshal result: " + err.Error())
	}
	return Response{OK: true, Result: data}
}

func errResponse(msg string) Response {
	return Response{OK: false, Error: msg}
}
