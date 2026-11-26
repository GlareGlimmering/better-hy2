package controller

import (
	"errors"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"h-ui/model/bo"
	"h-ui/model/vo"
	"h-ui/service"
)

const maxHysteria2ApplyResults = 16

type hysteria2ApplyResult struct {
	ID    string `json:"id"`
	State string `json:"state"`
	Error string `json:"error,omitempty"`
}

// HY2 operations can outlive an individual HTTP proxy request.
type hysteria2ApplyJobs struct {
	mu      sync.Mutex
	results []hysteria2ApplyResult
}

var configApplyJobs hysteria2ApplyJobs

func (jobs *hysteria2ApplyJobs) begin(apply func() error) (hysteria2ApplyResult, error) {
	jobs.mu.Lock()
	defer jobs.mu.Unlock()
	if n := len(jobs.results); n > 0 && jobs.results[n-1].State == "pending" {
		return hysteria2ApplyResult{}, errors.New("a Hysteria2 operation is already running")
	}
	id, err := uuid.NewRandom()
	if err != nil {
		return hysteria2ApplyResult{}, errors.New("could not create a Hysteria2 configuration operation")
	}
	result := hysteria2ApplyResult{ID: id.String(), State: "pending"}
	if len(jobs.results) == maxHysteria2ApplyResults {
		copy(jobs.results, jobs.results[1:])
		jobs.results = jobs.results[:maxHysteria2ApplyResults-1]
	}
	jobs.results = append(jobs.results, result)
	go func(result hysteria2ApplyResult) {
		result.State = "succeeded"
		if err := apply(); err != nil {
			result.State = "failed"
			result.Error = err.Error()
		}
		jobs.mu.Lock()
		jobs.results[len(jobs.results)-1] = result
		jobs.mu.Unlock()
	}(result)
	return result, nil
}

func (jobs *hysteria2ApplyJobs) status(id string) (hysteria2ApplyResult, bool) {
	jobs.mu.Lock()
	defer jobs.mu.Unlock()
	for _, result := range jobs.results {
		if result.ID == id {
			return result, true
		}
	}
	return hysteria2ApplyResult{}, false
}

func BeginHysteria2ConfigApply(c *gin.Context) {
	config, err := validateField(c, bo.Hysteria2ServerConfig{})
	if err != nil {
		return
	}
	beginHysteria2Job(c, func() error { return applyHysteria2Config(config) })
}

func ListFixedUDPRelays(c *gin.Context) {
	inventory, err := service.ListFixedUDPRelays()
	if err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	vo.Success(inventory, c)
}

func BeginAddFixedUDPRelay(c *gin.Context) {
	relay, err := validateField(c, service.FixedUDPRelay{})
	if err != nil {
		return
	}
	beginHysteria2Job(c, func() error { return service.ChangeFixedUDPRelay("", &relay) })
}

func BeginUpdateFixedUDPRelay(c *gin.Context) {
	request, err := validateField(c, struct {
		PreviousName string `json:"previousName" validate:"required,max=64"`
		service.FixedUDPRelay
	}{})
	if err != nil {
		return
	}
	if strings.TrimSpace(request.PreviousName) == "" {
		vo.Fail("UDP relay name is required", c)
		return
	}
	beginHysteria2Job(c, func() error { return service.ChangeFixedUDPRelay(request.PreviousName, &request.FixedUDPRelay) })
}

func BeginDeleteFixedUDPRelay(c *gin.Context) {
	request, err := validateField(c, struct {
		Name string `json:"name" validate:"required,max=64"`
	}{})
	if err != nil {
		return
	}
	beginHysteria2Job(c, func() error { return service.ChangeFixedUDPRelay(request.Name, nil) })
}

func beginHysteria2Job(c *gin.Context, operation func() error) {
	result, err := configApplyJobs.begin(operation)
	if err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	vo.Success(result, c)
}

func Hysteria2ConfigApplyStatus(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	result, ok := configApplyJobs.status(c.Query("id"))
	if !ok {
		vo.Fail("Hysteria2 configuration result unavailable", c)
		return
	}
	vo.Success(result, c)
}
