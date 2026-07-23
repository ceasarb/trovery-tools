package console

import (
	"fmt"
	"sync"
	"time"
)

// Spinner shows an animated indicator with elapsed time.
type Spinner struct {
	label   string
	start   time.Time
	done    chan struct{}
	stopped sync.Once
	wg      sync.WaitGroup
}

// StartSpinner begins an animated spinner with the given label.
func StartSpinner(label string) *Spinner {
	s := &Spinner{
		label: label,
		start: time.Now(),
		done:  make(chan struct{}),
	}
	s.wg.Add(1)
	go s.run()
	return s
}

func (s *Spinner) run() {
	defer s.wg.Done()

	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	i := 0
	for {
		select {
		case <-s.done:
			fmt.Print("\r\033[K")
			return
		case <-ticker.C:
			elapsed := time.Since(s.start).Round(100 * time.Millisecond)
			frame := frames[i%len(frames)]
			fmt.Printf("\r  %s %s %s",
				DimStyle.Render(frame),
				DimStyle.Render(s.label),
				DimStyle.Render(fmt.Sprintf("(%s)", elapsed)))
			i++
		}
	}
}

// Stop halts the spinner and clears the line. Blocks until the goroutine exits.
func (s *Spinner) Stop() {
	s.stopped.Do(func() {
		close(s.done)
		s.wg.Wait()
	})
}

// StopWithMessage halts the spinner and prints a final message.
func (s *Spinner) StopWithMessage(msg string) {
	s.stopped.Do(func() {
		close(s.done)
		s.wg.Wait()
		elapsed := time.Since(s.start).Round(time.Millisecond)
		fmt.Printf("  %s %s\n",
			DimStyle.Render(msg),
			DimStyle.Render(fmt.Sprintf("(%s)", elapsed)))
	})
}
