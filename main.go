// rsync custom agent for git-lfs

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"time"
)

var remote string

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	writer := bufio.NewWriter(os.Stdout)
	errWriter := bufio.NewWriter(os.Stderr)

	for scanner.Scan() {
		line := scanner.Text()
		var req request
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			writeToStderr(fmt.Sprintf("Unable to parse request: %v\n", line), errWriter)
			continue
		}

		switch req.Event {
		case "init":
			writeToStderr(fmt.Sprintf("Initialising rsync agent for: %s\n", req.Operation), errWriter)
			initAgent(writer, errWriter)
		case "download":
			writeToStderr(fmt.Sprintf("Received download request for: %s\n", req.Oid), errWriter)
			performDownload(req.Oid, req.Size, req.Action, writer, errWriter)
		case "upload":
			writeToStderr(fmt.Sprintf("Received upload request for: %s\n", req.Oid), errWriter)
			performUpload(req.Oid, req.Size, req.Action, req.Path, writer, errWriter)
		case "terminate":
			writeToStderr("Terminating rsync agent gracefully.\n", errWriter)
			// Nothing to do.
		}
	}
}

func writeToStderr(msg string, errWriter *bufio.Writer) {
	if !strings.HasSuffix(msg, "\n") {
		msg = msg + "\n"
	}
	errWriter.WriteString(msg)
	errWriter.Flush()
}

func sendResponse(r interface{}, writer, errWriter *bufio.Writer) error {
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	// Line oriented JSON
	b = append(b, '\n')
	_, err = writer.Write(b)
	if err != nil {
		return err
	}
	writer.Flush()
	writeToStderr(fmt.Sprintf("Sent message %v", string(b)), errWriter)
	return nil
}

func sendTransferError(oid string, code int, message string, writer, errWriter *bufio.Writer) {
	resp := &transferResponse{"complete", oid, "", &operationError{code, message}}
	err := sendResponse(resp, writer, errWriter)
	if err != nil {
		writeToStderr(fmt.Sprintf("Unable to send transfer error: %v\n", err), errWriter)
	}
}

func sendProgress(oid string, bytesSoFar int64, bytesSinceLast int, writer, errWriter *bufio.Writer) {
	resp := &progressResponse{"progress", oid, bytesSoFar, bytesSinceLast}
	err := sendResponse(resp, writer, errWriter)
	if err != nil {
		writeToStderr(fmt.Sprintf("Unable to send progress update: %v\n", err), errWriter)
	}
}

func initAgent(writer, errWriter *bufio.Writer) {
	// Make sure we have a remote.
	remote = os.Args[1]
	if remote == "" {
		resp := &initResponse{&operationError{3, "No remote specified when launching the process"}}
		sendResponse(resp, writer, errWriter)
		return
	}
	// Success!
	resp := &initResponse{}
	sendResponse(resp, writer, errWriter)
}

func performDownload(oid string, size int64, a *action, writer, errWriter *bufio.Writer) {
	dlFile, err := ioutil.TempFile("", "rsync-agent")
	if err != nil {
		sendTransferError(oid, 3, err.Error(), writer, errWriter)
		return
	}
	defer dlFile.Close()
	dlfilename := dlFile.Name()

	if err = rsync(remoteFile(oid), dlfilename); err != nil {
		sendTransferError(oid, 4, err.Error(), writer, errWriter)
		return
	}

	complete := &transferResponse{"complete", oid, dlfilename, nil}
	err = sendResponse(complete, writer, errWriter)
	if err != nil {
		writeToStderr(fmt.Sprintf("Unable to send completion message: %v\n", err), errWriter)
	}
}

func performUpload(oid string, size int64, a *action, fromPath string, writer, errWriter *bufio.Writer) {
	if err := rsync(fromPath, remoteFile(oid)); err != nil {
		sendTransferError(oid, 5, err.Error(), writer, errWriter)
		return
	}

	complete := &transferResponse{"complete", oid, "", nil}
	if err := sendResponse(complete, writer, errWriter); err != nil {
		writeToStderr(fmt.Sprintf("Unable to send completion message: %v\n", err), errWriter)
	}
}

func remoteFile(oid string) string {
	return fmt.Sprintf("%s/%s", remote, oid)
}

func rsync(args ...string) error {
	cmd := exec.Command("rsync", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Error while running `rsync %s`: %v\n%s", args, err, out)
	}
	return nil
}

type header struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}
type action struct {
	Href      string            `json:"href"`
	Header    map[string]string `json:"header,omitempty"`
	ExpiresAt time.Time         `json:"expires_at,omitempty"`
}
type operationError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Combined request struct which can accept anything
type request struct {
	Event               string  `json:"event"`
	Operation           string  `json:"operation"`
	Concurrent          bool    `json:"concurrent"`
	ConcurrentTransfers int     `json:"concurrenttransfers"`
	Oid                 string  `json:"oid"`
	Size                int64   `json:"size"`
	Path                string  `json:"path"`
	Action              *action `json:"action"`
}

type initResponse struct {
	Error *operationError `json:"error,omitempty"`
}
type transferResponse struct {
	Event string          `json:"event"`
	Oid   string          `json:"oid"`
	Path  string          `json:"path,omitempty"` // always blank for upload
	Error *operationError `json:"error,omitempty"`
}
type progressResponse struct {
	Event          string `json:"event"`
	Oid            string `json:"oid"`
	BytesSoFar     int64  `json:"bytesSoFar"`
	BytesSinceLast int    `json:"bytesSinceLast"`
}
