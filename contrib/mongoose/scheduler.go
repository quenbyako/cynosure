package goose

import (
	"context"
	"errors"
	"sync"
)

func RunJobs(ctx context.Context, jobs ...func(context.Context) error) error {
	jobCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var (
		errs []error
		wg   sync.WaitGroup
	)

	wg.Add(len(jobs))

	for i, job := range jobs {
		go func(_ int, job func(context.Context) error) {
			if err := omitContextErr(job(jobCtx)); err != nil {
				errs = append(errs, err)

				cancel()
			}

			wg.Done()
		}(i, job)
	}

	wg.Wait()

	return errors.Join(errs...)
}

func omitContextErr(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return nil
	}

	return err
}
