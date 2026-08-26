package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRESTServerDeploymentSmoke(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping process-level deployment smoke test")
	}

	temporary := t.TempDir()
	journeyFolder := filepath.Join(temporary, "journeys")
	scriptFolder := filepath.Join(temporary, "scripts")
	scheduleFolder := filepath.Join(temporary, "schedules")
	if err := os.MkdirAll(journeyFolder, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(scriptFolder, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(scheduleFolder, 0o755); err != nil {
		t.Fatal(err)
	}
	configFile := filepath.Join(temporary, "config.json")
	if err := os.WriteFile(configFile, []byte(`{
		"journey": {
			"base_url": "http://127.0.0.1",
			"rest_api": {
				"routes": {
					"journey_routes": ["/:realm/flows"],
					"journey_item_routes": ["/:realm/flows/:journeyId"],
					"script_routes": ["/:realm/js"],
					"script_item_routes": ["/:realm/js/:scriptId"],
					"step_schema_routes": ["/schemas"],
					"step_schema_item_routes": ["/schemas/:stepType"],
					"invoke_routes": ["/:realm/run"]
				}
			}
		}
	}`), 0o600); err != nil {
		t.Fatal(err)
	}

	binary := filepath.Join(temporary, "journey-rest-api")
	build := exec.Command("go", "build", "-o", binary, "./cmd/rest-api")
	build.Dir = "."
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build deployment command: %v\n%s", err, output)
	}

	address := availableRESTAddress(t)
	var processOutput bytes.Buffer
	command := exec.Command(binary,
		"-addr", address,
		"-config", configFile,
		"-journeys", journeyFolder,
		"-scripts", scriptFolder,
		"-schedules", scheduleFolder,
		"-encrypt-key", "0123456789abcdef0123456789abcdef",
	)
	command.Dir = temporary
	command.Stdout = &processOutput
	command.Stderr = &processOutput
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	defer stopRESTProcess(t, command, &processOutput)

	baseURL := "http://" + address
	waitForRESTHealth(t, baseURL)

	instances := deploymentRequest(t, http.MethodGet, baseURL+"/journey/smoke/instances?limit=500", nil)
	if instances.StatusCode != http.StatusOK || !strings.Contains(string(instances.Body), `"caches"`) {
		t.Fatalf("registered instances route status=%d body=%s", instances.StatusCode, instances.Body)
	}

	journey := map[string]any{
		"name": "Deployment smoke", "active": true, "default_exp": 1, "realm": "smoke",
		"start_step_id": "start", "sub_entries": []string{},
		"steps": map[string]any{"start": map[string]any{
			"name": "Complete", "step_type": "Success", "config": map[string]any{
				"data": map[string]any{"deployed": true},
			},
		}},
	}
	created := deploymentRequest(t, http.MethodPut, baseURL+"/journey/smoke/flows", journey)
	if created.StatusCode != http.StatusCreated {
		t.Fatalf("save status=%d body=%s", created.StatusCode, created.Body)
	}
	var stored struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(created.Body, &stored); err != nil || stored.ID == "" {
		t.Fatalf("created journey id=%q err=%v body=%s", stored.ID, err, created.Body)
	}

	executed := deploymentRequest(t, http.MethodPost, baseURL+"/journey/smoke/run", map[string]any{"journey_id": stored.ID})
	if executed.StatusCode != http.StatusOK || !strings.Contains(string(executed.Body), `"data"`) || strings.Contains(string(executed.Body), `"ctx"`) || strings.Contains(string(executed.Body), `"status"`) {
		t.Fatalf("execute status=%d body=%s", executed.StatusCode, executed.Body)
	}

	script := map[string]any{
		"name":        "deployment-schedule-script",
		"type":        "schedule",
		"code_base64": base64.StdEncoding.EncodeToString([]byte(`logger.Info("deployment schedule completed"); ({ done: true })`)),
	}
	createdScript := deploymentRequest(t, http.MethodPut, baseURL+"/journey/smoke/js", script)
	if createdScript.StatusCode != http.StatusCreated {
		t.Fatalf("save script status=%d body=%s", createdScript.StatusCode, createdScript.Body)
	}
	var storedScript struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createdScript.Body, &storedScript); err != nil || storedScript.ID == "" {
		t.Fatalf("created script id=%q err=%v body=%s", storedScript.ID, err, createdScript.Body)
	}
	schedule := map[string]any{
		"name":             "deployment-schedule",
		"active":           true,
		"kind":             "interval",
		"interval_seconds": 3600,
		"max_runs":         1,
		"trigger_enabled":  true,
		"trigger_wait":     true,
		"target": map[string]any{
			"type":        "script",
			"script_id":   storedScript.ID,
			"script_type": "schedule",
		},
	}
	createdSchedule := deploymentRequest(t, http.MethodPut, baseURL+"/journey/smoke/schedules", schedule)
	if createdSchedule.StatusCode != http.StatusCreated {
		t.Fatalf("save schedule status=%d body=%s", createdSchedule.StatusCode, createdSchedule.Body)
	}
	var storedSchedule struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createdSchedule.Body, &storedSchedule); err != nil || storedSchedule.ID == "" {
		t.Fatalf("created schedule id=%q err=%v body=%s", storedSchedule.ID, err, createdSchedule.Body)
	}
	listSchedules := deploymentRequest(t, http.MethodGet, baseURL+"/journey/smoke/schedules?name=deployment", nil)
	if listSchedules.StatusCode != http.StatusOK || !strings.Contains(string(listSchedules.Body), `"resultCount":1`) {
		t.Fatalf("list schedules status=%d body=%s", listSchedules.StatusCode, listSchedules.Body)
	}
	triggered := deploymentRequest(t, http.MethodPost, baseURL+"/journey/smoke/schedules/"+storedSchedule.ID+"/trigger", map[string]any{"wait": true})
	if triggered.StatusCode != http.StatusOK || !strings.Contains(string(triggered.Body), `"status":"succeeded"`) {
		t.Fatalf("trigger schedule status=%d body=%s", triggered.StatusCode, triggered.Body)
	}

	deleted := deploymentRequest(t, http.MethodDelete, baseURL+"/journey/smoke/flows/"+stored.ID, nil)
	if deleted.StatusCode != http.StatusNoContent {
		t.Fatalf("delete status=%d body=%s", deleted.StatusCode, deleted.Body)
	}
}

type deploymentResponse struct {
	StatusCode int
	Body       []byte
}

func deploymentRequest(t *testing.T, method, url string, body any) deploymentResponse {
	t.Helper()
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(encoded)
	}
	request, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 5 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return deploymentResponse{StatusCode: response.StatusCode, Body: data}
}

func availableRESTAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	return address
}

func waitForRESTHealth(t *testing.T, baseURL string) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	client := &http.Client{Timeout: 300 * time.Millisecond}
	for time.Now().Before(deadline) {
		response, err := client.Get(baseURL + "/health")
		if err == nil {
			response.Body.Close()
			if response.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("REST server did not become healthy")
}

func stopRESTProcess(t *testing.T, command *exec.Cmd, output *bytes.Buffer) {
	t.Helper()
	if command.Process == nil || command.ProcessState != nil {
		return
	}
	if err := command.Process.Signal(os.Interrupt); err != nil {
		_ = command.Process.Kill()
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("REST server shutdown: %v\n%s", err, output.String())
		}
	case <-time.After(5 * time.Second):
		_ = command.Process.Kill()
		<-done
		t.Errorf("REST server did not stop gracefully: %s", output.String())
	}
}
