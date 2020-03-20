package watcher

import (
	"context"
	consulapi "github.com/hashicorp/consul/api"
)

type ConfigEntryWatcher struct {
	client *consulapi.Client
	kind   string
	name   string
	ctx    context.Context
	cancel context.CancelFunc
}

func NewConfigEntryWatcher(
	client *consulapi.Client,
	kind string,
	name string,
) *ConfigEntryWatcher {
	ctx, cancel := context.WithCancel(context.Background())
	return &ConfigEntryWatcher{
		client: client,
		kind:   kind,
		name:   name,
		ctx:    ctx,
		cancel: cancel,
	}
}
func (w *ConfigEntryWatcher) Watch(ch chan consulapi.ConfigEntry) error {
	entry, meta, err := w.client.ConfigEntries().Get(w.kind, w.name, nil)
	if err != nil {
		return err
	}

	ch <- entry

	index := meta.LastIndex
	defer w.cancel()

	go (func() {
		opts := &consulapi.QueryOptions{
			WaitIndex: index,
		}
		entry, meta, err = w.client.ConfigEntries().Get(w.kind, w.name, opts.WithContext(w.ctx))
		if err != nil {
			return
		}
		if meta.LastIndex > index {
			ch <- entry
			index = meta.LastIndex
		}
	})()

	return nil
}

func (w *ConfigEntryWatcher) Cancel() {
	w.cancel()
}
