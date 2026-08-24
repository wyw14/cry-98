package notify

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/wyw14/cry-98/internal/model"
)

type Dispatcher struct {
	mu       sync.RWMutex
	channels map[string]Channel
	now      func() time.Time
}

func NewDispatcher(now func() time.Time, channels ...Channel) (*Dispatcher, error) {
	if now == nil {
		now = time.Now
	}
	dispatcher := &Dispatcher{channels: make(map[string]Channel), now: now}
	for _, channel := range channels {
		if channel == nil || channel.Name() == "" {
			return nil, errors.New("invalid notification channel")
		}
		if _, exists := dispatcher.channels[channel.Name()]; exists {
			return nil, errors.New("duplicate notification channel")
		}
		dispatcher.channels[channel.Name()] = channel
	}
	return dispatcher, nil
}

func (d *Dispatcher) Dispatch(ctx context.Context, alarm model.Alarm) ([]model.ChannelResult, error) {
	d.mu.RLock()
	channels := make([]Channel, 0, len(d.channels))
	for _, channel := range d.channels {
		channels = append(channels, channel)
	}
	d.mu.RUnlock()
	if len(channels) == 0 {
		return nil, errors.New("no notification channels configured")
	}
	results := make([]model.ChannelResult, len(channels))
	var wait sync.WaitGroup
	for index, channel := range channels {
		wait.Add(1)
		go func(index int, channel Channel) {
			defer wait.Done()
			err := channel.Send(ctx, alarm)
			result := model.ChannelResult{
				Channel:   channel.Name(),
				Required:  channel.Required(),
				Delivered: err == nil,
				Attempted: d.now().UTC(),
			}
			if err != nil {
				result.Error = err.Error()
			}
			results[index] = result
		}(index, channel)
	}
	wait.Wait()
	// A single failed required channel means the alarm as a whole was not
	// delivered. Surfacing that as an error keeps the alarm out of the
	// "delivered" state and hands the failed channels to the retry queue so
	// they keep being retried until they succeed. Optional channel failures are
	// still recorded per-channel but never block delivery.
	if failed := requiredFailures(results); len(failed) > 0 {
		return results, fmt.Errorf("required notification channels failed: %s", strings.Join(failed, ", "))
	}
	return results, nil
}

// requiredFailures returns the names of required channels that failed to
// deliver, preserving the order in which they appear in results.
func requiredFailures(results []model.ChannelResult) []string {
	var failed []string
	for _, result := range results {
		if result.Required && !result.Delivered {
			failed = append(failed, result.Channel)
		}
	}
	return failed
}

func (d *Dispatcher) Retry(ctx context.Context, alarm model.Alarm, previous []model.ChannelResult) ([]model.ChannelResult, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	results := append([]model.ChannelResult(nil), previous...)
	var firstErr error
	for index, previousResult := range previous {
		if previousResult.Delivered {
			continue
		}
		channel := d.channels[previousResult.Channel]
		if channel == nil {
			if firstErr == nil {
				firstErr = errors.New("notification channel no longer exists")
			}
			continue
		}
		err := channel.Send(ctx, alarm)
		results[index].Attempted = d.now().UTC()
		results[index].Delivered = err == nil
		results[index].Error = ""
		if err != nil {
			results[index].Error = err.Error()
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return results, firstErr
}
