package account

import "testing"

func Test_DefaultSharedAccount_ReadsEnv(t *testing.T) {
	t.Setenv("ARM_SUBSCRIPTION_ID", "00000000-0000-0000-0000-000000000000")
	acc := DefaultSharedAccount()
	if got := acc.GetSubscriptionId(); got == "" {
		t.Fatalf("expected subscription id from env, got empty")
	}
}
