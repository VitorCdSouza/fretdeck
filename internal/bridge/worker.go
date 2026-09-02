package bridge

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"os/exec"
	"sync"
)

// Worker is the python process that holds the input device open.
//
// One for the whole session. Opening a device costs a fraction of a second and
// doing it per note would be heard as a gap in what the tuner reports.
type Worker struct {
	Events chan Event

	mu    sync.Mutex
	stdin io.WriteCloser
	cmd   *exec.Cmd
}

func NewWorker() *Worker {
	return &Worker{Events: make(chan Event, 256)}
}

func (w *Worker) Start() error {
	path, err := scriptPath("worker.py")
	if err != nil {
		return err
	}

	// unbuffered, or a note would sit in the pipe until enough of them piled
	// up to be worth flushing
	cmd := exec.Command(python(), "-u", path)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}

	w.mu.Lock()
	w.cmd = cmd
	w.stdin = stdin
	w.mu.Unlock()

	go w.pump(stdout)
	go w.drain(stderr)

	return nil
}

func (w *Worker) pump(stdout io.Reader) {
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		var event Event
		if err := json.Unmarshal(line, &event); err != nil {
			w.Events <- Event{Event: EventLog, Message: string(line)}
			continue
		}
		w.Events <- event
	}

	// the output ended, which is a process that is gone. saying so is the only
	// thing between that and a screen waiting for an answer that cannot come
	w.buried()
	w.Events <- Event{Event: EventWorkerGone, Message: "the audio worker stopped"}
}

// buried reaps the process and takes the pipe away, so a send after it answers
// for itself instead of writing into a pipe nobody holds.
func (w *Worker) buried() {
	w.mu.Lock()
	stdin, cmd := w.stdin, w.cmd
	w.stdin, w.cmd = nil, nil
	w.mu.Unlock()

	if stdin != nil {
		_ = stdin.Close()
	}
	if cmd != nil {
		_ = cmd.Wait()
	}
}

// Running answers whether there is a process to talk to.
func (w *Worker) Running() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.stdin != nil
}

// drain turns whatever the libraries print on stderr into log events. Alsa
// prints on it every time a device is opened and none of it is fatal.
func (w *Worker) drain(stderr io.Reader) {
	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		w.Events <- Event{Event: EventLog, Message: scanner.Text()}
	}
}

func (w *Worker) Send(command Command) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.stdin == nil {
		return errors.New("the audio worker is not running")
	}

	line, err := json.Marshal(command)
	if err != nil {
		return err
	}

	_, err = w.stdin.Write(append(line, '\n'))
	return err
}

func (w *Worker) Close() {
	w.mu.Lock()
	stdin, cmd := w.stdin, w.cmd
	w.stdin, w.cmd = nil, nil
	w.mu.Unlock()

	if stdin != nil {
		_, _ = stdin.Write([]byte("{\"action\":\"quit\"}\n"))
		_ = stdin.Close()
	}
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}
}
