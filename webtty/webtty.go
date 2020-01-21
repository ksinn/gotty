package webtty

import (
	"context"
	"encoding/json"
	"sync"
	//"log"
	"github.com/pkg/errors"

	"github.com/ksinn/gotty/file"
)

// WebTTY bridges a PTY slave and its PTY master.
// To support text-based streams and side channel commands such as
// terminal resizing, WebTTY uses an original protocol.
type WebTTY struct {
	// PTY Master, which probably a connection to browser
	masterConn Master
	// PTY Slave
	slave Slave

	windowTitle []byte
	permitWrite bool
	columns     int
	rows        int
	reconnect   int // in seconds
	masterPrefs []byte

	bufferSize int
	writeMutex sync.Mutex
}

// New creates a new instance of WebTTY.
// masterConn is a connection to the PTY master,
// typically it's a websocket connection to a client.
// slave is a PTY slave such as a local command with a PTY.
func New(masterConn Master, slave Slave, options ...Option) (*WebTTY, error) {
	wt := &WebTTY{
		masterConn: masterConn,
		slave:      slave,

		permitWrite: false,
		columns:     0,
		rows:        0,

		bufferSize: 1024,
	}

	for _, option := range options {
		option(wt)
	}

	return wt, nil
}

// Run starts the main process of the WebTTY.
// This method blocks until the context is canceled.
// Note that the master and slave are left intact even
// after the context is canceled. Closing them is caller's
// responsibility.
// If the connection to one end gets closed, returns ErrSlaveClosed or ErrMasterClosed.
func (wt *WebTTY) Run(ctx context.Context) (err error) {
	err = wt.sendInitializeMessage()
	if err != nil {
		return errors.Wrapf(err, "failed to send initializing message")
	}

	errs := make(chan error, 2)

	go func() {
		errs <- func() error {
			buffer := make([]byte, wt.bufferSize)
			for {
				n, err := wt.slave.Read(buffer)
				if err != nil {
					return ErrSlaveClosed
				}

				err = wt.handleSlaveReadEvent(buffer[:n])
				if err != nil {
					return err
				}
			}
		}()
	}()

	go func() {
		errs <- func() error {
			buffer := make([]byte, wt.bufferSize)
			for {
				n, err := wt.masterConn.Read(buffer)
				if err != nil {
					return ErrMasterClosed
				}

				err = wt.handleMasterReadEvent(buffer[:n])
				if err != nil {
					return err
				}
			}
		}()
	}()

	select {
	case <-ctx.Done():
		err = ctx.Err()
	case err = <-errs:
	}

	return err
}

func (wt *WebTTY) sendInitializeMessage() error {

	dirContent, err := file.GetDirContent()
	if err != nil {
		return errors.Wrapf(err, "failed to send dir content")
	}

	jsonDirContent, err := json.Marshal(dirContent)
	if err != nil {
		return errors.Wrapf(err, "failed to send dir content")
	}

	err = wt.masterWrite(append([]byte{ListOfFile}, jsonDirContent...))
	if err != nil {
		return errors.Wrapf(err, "failed to send window title")
	}

	//if wt.reconnect > 0 {
	//	reconnect, _ := json.Marshal(wt.reconnect)
	//	err := wt.masterWrite(append([]byte{SetReconnect}, reconnect...))
	//	if err != nil {
	//		return errors.Wrapf(err, "failed to set reconnect")
	//	}
	//}
	//
	//if wt.masterPrefs != nil {
	//	err := wt.masterWrite(append([]byte{SetPreferences}, wt.masterPrefs...))
	//	if err != nil {
	//		return errors.Wrapf(err, "failed to set preferences")
	//	}
	//}

	return nil
}

func (wt *WebTTY) handleSlaveReadEvent(data []byte) error {

	err := wt.masterWrite(append([]byte{Output}, data...))
	if err != nil {
		return errors.Wrapf(err, "failed to send message to master")
	}

	return nil
}

func (wt *WebTTY) masterWrite(data []byte) error {

	wt.writeMutex.Lock()
	defer wt.writeMutex.Unlock()

	_, err := wt.masterConn.Write(data)
	if err != nil {
		return errors.Wrapf(err, "failed to write to master")
	}

	return nil
}

func (wt *WebTTY) handleMasterReadEvent(data []byte) error {

	if len(data) == 0 {
		return errors.New("unexpected zero length read from master")
	}

	//_, err := wt.slave.Write(data[1:])
	//if err != nil {
	//	return errors.Wrapf(err, "failed to write received data to slave")
	//}

	if !wt.permitWrite {
		return nil
	}


	//Отправляет весь вход к терменалу
	//_, err := wt.slave.Write(data)
	//if err != nil {
	//	return errors.Wrapf(err, "failed to write received data to slave")
	//}
	//
	//return nil

	switch data[0] {
	case Input:
		if !wt.permitWrite {
			return nil
		}

		if len(data) <= 1 {
			return nil
		}

		_, err := wt.slave.Write(data[1:])
		if err != nil {
			return errors.Wrapf(err, "failed to write received data to slave")
		}

	case Ping:
		err := wt.masterWrite([]byte{Pong})
		if err != nil {
			return errors.Wrapf(err, "failed to return Pong message to master")
		}

	case ResizeTerminal:
		if wt.columns != 0 && wt.rows != 0 {
			break
		}

		if len(data) <= 1 {
			return errors.New("received malformed remote command for terminal resize: empty payload")
		}

		var args argResizeTerminal
		err := json.Unmarshal(data[1:], &args)
		if err != nil {
			return errors.Wrapf(err, "received malformed data for terminal resize")
		}
		rows := wt.rows
		if rows == 0 {
			rows = int(args.Rows)
		}

		columns := wt.columns
		if columns == 0 {
			columns = int(args.Columns)
		}

		wt.slave.ResizeTerminal(columns, rows)

	case WriteFile:
		if len(data) <= 1 {
			return errors.New("received malformed remote command for write file: empty payload")
		}

		var args argWriteFie
		err := json.Unmarshal(data[1:], &args)
		if err != nil {
			return errors.Wrapf(err, "received malformed data for write file")
		}

		path := args.Path
		content := args.Content

		err = file.WriteFile(path, content)
		if err != nil {
			return errors.Wrapf(err, "error with writing file")
		}

	case RemoveFile:
		if len(data) <= 1 {
			return errors.New("received malformed remote command for remove file: empty payload")
		}

		path := string(data[1:])

		err := file.RemoveFile(path)
		if err != nil {
			return errors.Wrapf(err, "error with removing file")
		}

	default:
		return errors.Errorf("unknown message type `%c`", data[0])
	}

	return nil
}

type argResizeTerminal struct {
	Columns float64
	Rows    float64
}

type argWriteFie struct {
	Path string
	Content    []byte
}

