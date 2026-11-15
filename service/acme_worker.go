package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"h-ui/model/bo"
)

var acmeWorkerTokenPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}\.[a-f0-9]{64}$`)

func validateACMEWorker(config bo.Hysteria2ServerConfig, kernelPath string) error {
	acme := config.ACME
	if acme == nil || acme.Type == nil || !strings.EqualFold(*acme.Type, "dns") ||
		acme.DNS == nil || acme.DNS.Name == nil || !strings.EqualFold(*acme.DNS.Name, "worker") {
		return nil
	}

	values := acme.DNS.Config
	endpoint, err := url.Parse(values["worker_url"])
	if err != nil || endpoint.Scheme != "https" || endpoint.Hostname() == "" ||
		endpoint.User != nil || endpoint.RawQuery != "" || endpoint.ForceQuery || endpoint.Fragment != "" ||
		(endpoint.EscapedPath() != "" && endpoint.EscapedPath() != "/") {
		return errors.New("ACME worker_url must be an HTTPS origin")
	}
	if !acmeWorkerTokenPattern.MatchString(values["worker_token"]) {
		return errors.New("invalid ACME worker_token: expected node ID and 64 lowercase hexadecimal digits")
	}
	if len(values) != 2 {
		return errors.New("ACME Worker configuration accepts only worker_url and worker_token")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, kernelPath, "version", "--json").Output()
	var capabilities struct {
		ACMEDNSWorker int `json:"acmeDNSWorker"`
	}
	if err != nil || json.Unmarshal(output, &capabilities) != nil || capabilities.ACMEDNSWorker != 1 {
		return errors.New("installed Hysteria2 kernel does not support the ACME Worker provider; install a compatible kernel before applying this configuration")
	}
	return nil
}
