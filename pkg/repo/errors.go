package repo

import (
	"errors"
	"fmt"
)

var (
	ErrNameEmpty = errors.New("repository name is empty")
	ErrURLEmpty  = errors.New("repository url is empty")
)

type InvalidURLError struct {
	URL string
}

func (err InvalidURLError) Error() string {
	return fmt.Sprintf("invalid URL: %s", err.URL)
}

//nolint:errorlint
func (InvalidURLError) Is(target error) bool {
	switch target.(type) {
	case InvalidURLError, *InvalidURLError:
		return true
	default:
		return false
	}
}

type DuplicateError struct {
	Name string
}

func (err DuplicateError) Error() string {
	return fmt.Sprintf("repository duplicate: %s", err.Name)
}

//nolint:errorlint
func (DuplicateError) Is(target error) bool {
	switch target.(type) {
	case DuplicateError, *DuplicateError:
		return true
	default:
		return false
	}
}

type NotFoundError struct {
	Name string
}

func (err NotFoundError) Error() string {
	return fmt.Sprintf("🗄 repository not found: %s", err.Name)
}

//nolint:errorlint
func (NotFoundError) Is(target error) bool {
	switch target.(type) {
	case NotFoundError, *NotFoundError:
		return true
	default:
		return false
	}
}