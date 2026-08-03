package contracttest_test

import (
	"os"
	"testing"

	"github.com/thoriqakbar0/garden/internal/contracttest"
)

func TestOfficialEveConversationContract(t *testing.T) {
	baseURL := os.Getenv("EVE_OFFICIAL_BASE_URL")
	if baseURL == "" {
		t.Skip("set EVE_OFFICIAL_BASE_URL to the official agent-workflow-stress fixture")
	}
	contracttest.RunConversationContract(t, baseURL)
}

func TestOfficialEveCancellationContract(t *testing.T) {
	baseURL := os.Getenv("EVE_OFFICIAL_CANCELLATION_BASE_URL")
	if baseURL == "" {
		t.Skip("set EVE_OFFICIAL_CANCELLATION_BASE_URL to the official agent-cancellation fixture")
	}
	contracttest.RunCancellationContract(t, baseURL)
}
