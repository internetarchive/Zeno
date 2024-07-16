package queue

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"net/url"

	protobufv1 "github.com/internetarchive/Zeno/internal/pkg/queue/protobuf/v1"
	"google.golang.org/protobuf/proto"
)

func NewItem(URL *url.URL, parentItem *Item, itemType string, hop uint64, ID string, bypassSeencheck bool) (*Item, error) {
	urlJSON, err := json.Marshal(URL)
	if err != nil {
		return nil, err
	}

	var parentItemBytes []byte
	if parentItem != nil {
		parentItemBytes, err = proto.Marshal(parentItem.ProtoItem)
		if err != nil {
			return nil, err
		}
	}

	protoItem := &protobufv1.ProtoItem{
		Url:             urlJSON,
		ID:              ID,
		Host:            URL.Host,
		Hop:             hop,
		Type:            itemType,
		BypassSeencheck: bypassSeencheck,
		ParentItem:      parentItemBytes,
	}

	h := fnv.New64a()
	h.Write([]byte(URL.String()))
	protoItem.Hash = h.Sum64()

	return &Item{
		ProtoItem: protoItem,
		URL:       URL,
		Parent:    parentItem,
	}, nil
}

func (q *PersistentGroupedQueue) ReadItemAt(position uint64, itemSize uint64) ([]byte, error) {
	// Ensure the file pointer is at the correct position
	_, err := q.queueFile.Seek(int64(position), io.SeekStart)
	if err != nil {
		return nil, fmt.Errorf("failed to seek to item position: %w", err)
	}

	// Read the specific number of bytes for the item
	itemBytes := make([]byte, itemSize)
	_, err = io.ReadFull(q.queueFile, itemBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to read item bytes: %w", err)
	}

	return itemBytes, nil
}

func (i *Item) UnmarshalParent() error {
	if i.ParentItem == nil || len(i.ParentItem) == 0 {
		return nil
	}

	parentProtoItem := &protobufv1.ProtoItem{}
	err := proto.Unmarshal(i.ParentItem, parentProtoItem)
	if err != nil {
		return err
	}

	var parentURL url.URL
	err = json.Unmarshal(parentProtoItem.Url, &parentURL)
	if err != nil {
		return err
	}

	i.Parent = &Item{
		ProtoItem: parentProtoItem,
		URL:       &parentURL,
	}

	return i.Parent.UnmarshalParent()
}
