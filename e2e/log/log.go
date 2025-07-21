package log

import (
	"fmt"
	"io"
	"testing"

	"github.com/go-logfmt/logfmt"
)

type RecordMatcher interface {
	Match(record map[string]string)
	Assert(t *testing.T)
	ShouldStop() bool
}

func ParseLog(r io.Reader) chan map[string]string {
	d := logfmt.NewDecoder(r)
	out := make(chan map[string]string)

	go func() {
		defer close(out)
		for d.ScanRecord() {
			record := make(map[string]string)
			for d.ScanKeyval() {
				if _, ok := record[string(d.Key())]; ok {
					panic(fmt.Sprintf("duplicate key %s in record", d.Key()))
				}
				record[string(d.Key())] = string(d.Value())
			}
			if !hasKey(record, "level") || !hasKey(record, "time") {
				fmt.Printf("ignore record without level or time: %v\n", record)
				continue
			}
			out <- record
		}
		if d.Err() != nil {
			panic(d.Err())
		}
	}()
	return out
}

func LogRecordProcessor(pipeR *io.PipeReader, rm RecordMatcher, stopCh chan struct{}) error {
	chClosed := false
	logCh := ParseLog(pipeR)
	for record := range logCh {
		rm.Match(record)
		if !chClosed && rm.ShouldStop() {
			close(stopCh)
			chClosed = true
		}
		fmt.Printf("log record: %v\n", record)
	}
	return nil
}
