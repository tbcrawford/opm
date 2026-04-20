package symlink

import "time"

func retrySwap(attempts int, sleep func(time.Duration), op func() error, isRetryable func(error) bool) error {
	for attempt := 0; attempt < attempts; attempt++ {
		if err := op(); err == nil {
			return nil
		} else if !isRetryable(err) || attempt == attempts-1 {
			return err
		}

		delay := time.Duration(attempt+1) * 25 * time.Millisecond
		if sleep != nil {
			sleep(delay)
		} else {
			time.Sleep(delay)
		}
	}

	return nil
}
