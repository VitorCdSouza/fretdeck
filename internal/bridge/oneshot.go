package bridge

import (
	"bufio"
	"context"
	"encoding/json"
	"os/exec"
)

// Run starts a script that has one job, and streams what it says until it
// ends. The importer and the analyzer are both like that: they answer once and
// exit, so there is nothing to keep alive between two of them.
func Run(ctx context.Context, script string, args []string, events chan<- Event) error {
	path, err := scriptPath(script)
	if err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, python(), append([]string{"-u", path}, args...)...)

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

	done := make(chan struct{})
	go func() {
		defer close(done)
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			events <- Event{Event: EventLog, Message: scanner.Text()}
		}
	}()

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for scanner.Scan() {
		var event Event
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			events <- Event{Event: EventLog, Message: scanner.Text()}
			continue
		}
		events <- event
	}

	<-done
	return cmd.Wait()
}
