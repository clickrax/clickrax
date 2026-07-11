package pbs

import (
	"encoding/json"
	"testing"
)

func TestParseTaskStatusResponse(t *testing.T) {
	body := []byte(`{"data":{"status":"stopped","exitstatus":"OK"}}`)
	var result struct {
		Data struct {
			Status     string `json:"status"`
			ExitStatus string `json:"exitstatus"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatal(err)
	}
	if result.Data.Status != "stopped" || result.Data.ExitStatus != "OK" {
		t.Fatalf("%+v", result.Data)
	}
}
