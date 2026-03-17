package daemon

import (
	"encoding/json"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/byronxlg/agentlog/internal/store"
)

func (d *Daemon) handleRequest(req Request) Response {
	switch req.Method {
	case "write":
		return d.handleWrite(req.Params)
	case "query":
		return d.handleQuery(req.Params)
	case "search":
		return d.handleSearch(req.Params)
	case "get_session":
		return d.handleGetSession(req.Params)
	case "list_sessions":
		return d.handleListSessions()
	case "create_session":
		return d.handleCreateSession()
	case "blame":
		return d.handleBlame(req.Params)
	case "context":
		return d.handleContext(req.Params)
	case "export":
		return d.handleExport(req.Params)
	default:
		return errResponse("unknown method: " + req.Method)
	}
}

type writeResult struct {
	entry store.Entry
	err   error
}

type writeRequest struct {
	entry  store.Entry
	result chan writeResult
}

func (d *Daemon) handleWrite(params json.RawMessage) Response {
	var p WriteParams
	if err := json.Unmarshal(params, &p); err != nil {
		return errResponse("invalid write params: " + err.Error())
	}

	entry := store.Entry{
		ID:        p.Entry.ID,
		SessionID: p.Entry.SessionID,
		Type:      store.EntryType(p.Entry.Type),
		Title:     p.Entry.Title,
		Body:      p.Entry.Body,
		Tags:      p.Entry.Tags,
		FileRefs:  p.Entry.FileRefs,
	}

	if entry.ID == "" {
		entry.ID = uuid.New().String()
	}
	if p.Entry.Timestamp != "" {
		t, err := time.Parse(time.RFC3339Nano, p.Entry.Timestamp)
		if err != nil {
			return errResponse("invalid timestamp: " + err.Error())
		}
		entry.Timestamp = t
	} else {
		entry.Timestamp = time.Now().UTC()
	}

	ch := make(chan writeResult, 1)
	select {
	case d.writeCh <- writeRequest{entry: entry, result: ch}:
	case <-d.done:
		return errResponse("daemon shutting down")
	}

	select {
	case res := <-ch:
		if res.err != nil {
			return errResponse("write failed: " + res.err.Error())
		}
		return okResponse(res.entry)
	case <-d.done:
		return errResponse("daemon shutting down")
	}
}

func (d *Daemon) handleQuery(params json.RawMessage) Response {
	var p QueryParams
	if err := json.Unmarshal(params, &p); err != nil {
		return errResponse("invalid query params: " + err.Error())
	}

	var entries []store.Entry
	var err error

	switch {
	case p.Start != "" && p.End != "":
		start, parseErr := time.Parse(time.RFC3339Nano, p.Start)
		if parseErr != nil {
			return errResponse("invalid start time: " + parseErr.Error())
		}
		end, parseErr := time.Parse(time.RFC3339Nano, p.End)
		if parseErr != nil {
			return errResponse("invalid end time: " + parseErr.Error())
		}
		entries, err = d.index.QueryByTimeRange(start, end)
	case p.Type != "":
		entries, err = d.index.QueryByType(store.EntryType(p.Type))
	case p.SessionID != "":
		entries, err = d.index.QueryBySession(p.SessionID)
	case len(p.Tags) > 0:
		entries, err = d.index.QueryByTags(p.Tags)
	case p.FilePath != "":
		entries, err = d.index.QueryByFilePath(p.FilePath)
	default:
		limit := p.Limit
		if limit <= 0 {
			limit = 50
		}
		offset := p.Offset
		if offset < 0 {
			offset = 0
		}
		entries, err = d.index.QueryRecent(limit, offset)
	}

	if err != nil {
		return errResponse("query failed: " + err.Error())
	}
	return okResponse(entries)
}

func (d *Daemon) handleSearch(params json.RawMessage) Response {
	var p SearchParams
	if err := json.Unmarshal(params, &p); err != nil {
		return errResponse("invalid search params: " + err.Error())
	}
	if p.Query == "" {
		return errResponse("search query must not be empty")
	}

	entries, err := d.index.Search(p.Query)
	if err != nil {
		return errResponse("search failed: " + err.Error())
	}
	return okResponse(entries)
}

func (d *Daemon) handleGetSession(params json.RawMessage) Response {
	var p GetSessionParams
	if err := json.Unmarshal(params, &p); err != nil {
		return errResponse("invalid get_session params: " + err.Error())
	}
	if p.SessionID == "" {
		return errResponse("session_id must not be empty")
	}

	entries, err := d.store.ReadSession(p.SessionID)
	if err != nil {
		return errResponse("get session failed: " + err.Error())
	}
	return okResponse(entries)
}

func (d *Daemon) handleListSessions() Response {
	sessions, err := d.store.ListSessions()
	if err != nil {
		return errResponse("list sessions failed: " + err.Error())
	}
	return okResponse(sessions)
}

func (d *Daemon) handleCreateSession() Response {
	id := d.createSession()
	return okResponse(map[string]string{"session_id": id})
}

func (d *Daemon) handleBlame(params json.RawMessage) Response {
	var p BlameParams
	if err := json.Unmarshal(params, &p); err != nil {
		return errResponse("invalid blame params: " + err.Error())
	}
	if p.FilePath == "" {
		return errResponse("file_path must not be empty")
	}

	entries, err := d.index.QueryByFilePath(p.FilePath)
	if err != nil {
		return errResponse("blame failed: " + err.Error())
	}
	return okResponse(entries)
}

func (d *Daemon) handleContext(params json.RawMessage) Response {
	var p ContextParams
	if err := json.Unmarshal(params, &p); err != nil {
		return errResponse("invalid context params: " + err.Error())
	}
	if len(p.Files) == 0 && p.Topic == "" {
		return errResponse("context requires at least one of files or topic")
	}

	limit := p.Limit
	if limit <= 0 {
		limit = 10
	}

	seen := make(map[string]bool)
	var entries []store.Entry

	for _, f := range p.Files {
		results, err := d.index.QueryByFilePath(f)
		if err != nil {
			return errResponse("context file lookup failed: " + err.Error())
		}
		for _, e := range results {
			if !seen[e.ID] {
				seen[e.ID] = true
				entries = append(entries, e)
			}
		}
	}

	if p.Topic != "" {
		results, err := d.index.Search(p.Topic)
		if err != nil {
			return errResponse("context topic search failed: " + err.Error())
		}
		for _, e := range results {
			if !seen[e.ID] {
				seen[e.ID] = true
				entries = append(entries, e)
			}
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp.After(entries[j].Timestamp)
	})

	if len(entries) > limit {
		entries = entries[:limit]
	}

	return okResponse(entries)
}
