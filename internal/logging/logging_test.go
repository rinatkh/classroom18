package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestLevelFilteringAndFatalName(t *testing.T) {
	for _, level := range []string{"", "INFO", "DEBUG", "ERROR", "FATAL"} {
		t.Run(level, func(t *testing.T) {
			var output bytes.Buffer
			logger, err := New(&output, "test", level, "json")
			if err != nil {
				t.Fatal(err)
			}
			logger.Debug("detail")
			logger.Info("started")
			logger.Error("failed")
			logger.Log(context.Background(), LevelFatal, "cannot continue")
			var levels []string
			decoder := json.NewDecoder(&output)
			for decoder.More() {
				var record struct {
					Level   string `json:"level"`
					Service string `json:"service"`
				}
				if err := decoder.Decode(&record); err != nil {
					t.Fatal(err)
				}
				if record.Service != "test" {
					t.Fatalf("service=%q", record.Service)
				}
				levels = append(levels, record.Level)
			}
			want := "INFO,ERROR,FATAL"
			switch level {
			case "DEBUG":
				want = "DEBUG,INFO,ERROR,FATAL"
			case "ERROR":
				want = "ERROR,FATAL"
			case "FATAL":
				want = "FATAL"
			}
			if strings.Join(levels, ",") != want {
				t.Fatalf("levels=%v, want %s", levels, want)
			}
		})
	}
	for _, config := range [][2]string{{"TRACE", "text"}, {"INFO", "xml"}} {
		if _, err := New(&bytes.Buffer{}, "test", config[0], config[1]); err == nil {
			t.Fatal("invalid config accepted")
		}
	}
}

func TestErrorDoesNotExposeDriverDetails(t *testing.T) {
	var output bytes.Buffer
	logger, err := New(&output, "test", "DEBUG", "json")
	if err != nil {
		t.Fatal(err)
	}
	previous := slog.Default()
	slog.SetDefault(logger)
	t.Cleanup(func() { slog.SetDefault(previous) })
	Error(context.Background(), "repository failed", errors.New("password=private-password token=private-token"), "task_id", int64(9))
	if strings.Contains(output.String(), "private-") {
		t.Fatal("raw error details leaked")
	}
	var record map[string]any
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatal(err)
	}
	if record["level"] != "ERROR" || record["task_id"] != float64(9) || record["error_type"] == nil {
		t.Fatalf("record=%v", record)
	}
}

// subprocess позволяет проверить настоящий exit code, не завершая go test.
func TestFatalProcess(t *testing.T) {
	if os.Getenv("LOGGING_FATAL_CHILD") == "1" {
		if err := Configure("fatal-test"); err != nil {
			panic(err)
		}
		Fatal("cannot start", "reason", "test failure")
		return
	}
	command := exec.Command(os.Args[0], "-test.run=^TestFatalProcess$")
	command.Env = append(os.Environ(), "LOGGING_FATAL_CHILD=1", "LOG_LEVEL=FATAL", "LOG_FORMAT=json")
	output, err := command.CombinedOutput()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 1 {
		t.Fatalf("exit=%v output=%s", err, output)
	}
	var record map[string]any
	if err := json.Unmarshal(output, &record); err != nil {
		t.Fatalf("invalid FATAL JSON: %v %s", err, output)
	}
	if record["level"] != "FATAL" || record["msg"] != "cannot start" {
		t.Fatalf("record=%v", record)
	}
}
