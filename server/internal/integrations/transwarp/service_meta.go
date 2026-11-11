package transwarp

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type MetaEntry struct {
	Key  string `json:"key"`
	Name string `json:"name"`
	Type string `json:"type"`
}

type metaTreeEntry struct {
	Key      string          `json:"key"`
	Title    string          `json:"title"`
	Type     string          `json:"type"`
	Children []metaTreeEntry `json:"children"`
	Leaf     bool            `json:"isLeaf"`
}

type MetaFile struct {
	Key     string `json:"key"`
	Text    bool   `json:"text"`
	Content string `json:"content"`
}

func (c *Client) ServiceMetaFiles(ctx context.Context, serviceID int64) ([]MetaEntry, error) {
	if serviceID <= 0 {
		return nil, errors.New("service_id is required")
	}
	var service rawService
	if err := c.get(ctx, fmt.Sprintf("/api/services/%d", serviceID), nil, &service); err != nil {
		return nil, err
	}
	if service.ID != serviceID {
		return nil, fmt.Errorf("%w: Manager returned a different service", ErrResponse)
	}
	for _, segment := range []string{service.Type, service.Version} {
		if segment == "" || segment == "." || segment == ".." || strings.ContainsAny(segment, "/\\\x00") {
			return nil, fmt.Errorf("%w: Manager returned an invalid service type or version", ErrResponse)
		}
	}
	var tree []metaTreeEntry
	if err := c.get(ctx, "/api/toolbox/metaEditor/tree/service", url.Values{"type": {service.Type}, "version": {service.Version}}, &tree); err != nil {
		return nil, err
	}
	files := []MetaEntry{}
	prefix := service.Type + "/" + service.Version + "/"
	var visit func([]metaTreeEntry)
	visit = func(entries []metaTreeEntry) {
		for _, entry := range entries {
			if entry.Leaf && strings.HasPrefix(entry.Key, prefix) {
				files = append(files, MetaEntry{entry.Key, strings.TrimPrefix(entry.Key, prefix), entry.Type})
			}
			visit(entry.Children)
		}
	}
	visit(tree)
	return files, nil
}

func (c *Client) serviceMetaEntry(ctx context.Context, serviceID int64, key string) (MetaEntry, error) {
	files, err := c.ServiceMetaFiles(ctx, serviceID)
	if err != nil {
		return MetaEntry{}, err
	}
	for _, file := range files {
		if file.Key == key {
			return file, nil
		}
	}
	return MetaEntry{}, errors.New("file does not belong to this service version")
}

func (c *Client) ServiceMetaFile(ctx context.Context, serviceID int64, key string) (MetaFile, error) {
	entry, err := c.serviceMetaEntry(ctx, serviceID, key)
	if err != nil {
		return MetaFile{}, err
	}
	return c.readServiceMetaFile(ctx, entry)
}

func (c *Client) readServiceMetaFile(ctx context.Context, entry MetaEntry) (MetaFile, error) {
	var raw struct {
		Text    *bool   `json:"text"`
		Content *string `json:"content"`
	}
	if err := c.get(ctx, "/api/toolbox/metaEditor/file/text", url.Values{"category": {"SERVICE"}, "type": {entry.Type}, "key": {entry.Key}}, &raw); err != nil {
		return MetaFile{}, err
	}
	if raw.Text == nil || (*raw.Text && raw.Content == nil) {
		return MetaFile{}, fmt.Errorf("%w: Manager metainfo file response is missing text content", ErrResponse)
	}
	file := MetaFile{Key: entry.Key, Text: *raw.Text}
	if raw.Content != nil {
		file.Content = *raw.Content
	}
	return file, nil
}

func (c *Client) SaveServiceMetaFile(ctx context.Context, serviceID int64, key, content string) error {
	entry, err := c.serviceMetaEntry(ctx, serviceID, key)
	if err != nil {
		return err
	}
	current, err := c.readServiceMetaFile(ctx, entry)
	if err != nil {
		return err
	}
	if !current.Text {
		return errors.New("only text metainfo files can be edited")
	}
	return c.request(ctx, http.MethodPost, "/api/toolbox/metaEditor/file/text", nil, map[string]string{"category": "SERVICE", "type": entry.Type, "key": key, "content": content}, nil)
}
