package grapery_client

import (
	"errors"
	"fmt"
)

var ErrNoActiveWorkflow = errors.New("no_active_workflow")

type PolicyAPIError struct {
	Path               string
	Status             int
	Code               int
	Message, ErrorCode string
}

func (e *PolicyAPIError) Error() string {
	return fmt.Sprintf("grapery policy api %s: status %d: %s", e.Path, e.Status, e.Message)
}
func (e *PolicyAPIError) Is(target error) bool {
	return target == ErrNoActiveWorkflow && e.Status == 404 && (e.ErrorCode == "no_active_workflow" || (e.ErrorCode == "" && e.Message == "no_active_workflow"))
}
