package account

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
)

type ResourceManagerAccount struct {
	subscriptionId *string
	mutex          *sync.Mutex
}

var account *ResourceManagerAccount

func NewResourceManagerAccount() ResourceManagerAccount {
	var subscriptionId *string
	if v := os.Getenv("ARM_SUBSCRIPTION_ID"); v != "" {
		subscriptionId = &v
	}
	return ResourceManagerAccount{
		mutex:          &sync.Mutex{},
		subscriptionId: subscriptionId,
	}
}

func DefaultSharedAccount() ResourceManagerAccount {
	if account == nil {
		v := NewResourceManagerAccount()
		account = &v
	}
	return *account
}

func (account *ResourceManagerAccount) GetSubscriptionId() string {
	account.mutex.Lock()
	defer account.mutex.Unlock()

	if account.subscriptionId != nil {
		return *account.subscriptionId
	}

	err := account.loadDefaultsFromAzCmd()
	if err != nil {
		log.Printf("[DEBUG] Error getting default subscription ID: %s", err)
	}

	if account.subscriptionId == nil {
		log.Printf("[DEBUG] No subscription ID found")
		return ""
	}

	return *account.subscriptionId
}

func (account *ResourceManagerAccount) loadDefaultsFromAzCmd() error {
	var accountModel struct {
		SubscriptionID string `json:"id"`
		TenantId       string `json:"tenantId"`
	}
	err := jsonUnmarshalAzCmd(&accountModel, "account", "show")
	if err != nil {
		return fmt.Errorf("obtaining defaults from az cmd: %s", err)
	}

	account.subscriptionId = &accountModel.SubscriptionID
	return nil
}

// jsonUnmarshalAzCmd executes an Azure CLI command and unmarshalls the JSON output.
func jsonUnmarshalAzCmd(i interface{}, arg ...string) error {
	var stderr bytes.Buffer
	var stdout bytes.Buffer

	arg = append(arg, "-o=json")
	cmd := exec.Command("az", arg...)
	cmd.Stderr = &stderr
	cmd.Stdout = &stdout

	if err := cmd.Start(); err != nil {
		err := fmt.Errorf("launching Azure CLI: %+v", err)
		if stdErrStr := stderr.String(); stdErrStr != "" {
			err = fmt.Errorf("%s: %s", err, strings.TrimSpace(stdErrStr))
		}
		return err
	}

	if err := cmd.Wait(); err != nil {
		err := fmt.Errorf("running Azure CLI: %+v", err)
		if stdErrStr := stderr.String(); stdErrStr != "" {
			err = fmt.Errorf("%s: %s", err, strings.TrimSpace(stdErrStr))
		}
		return err
	}

	if err := json.Unmarshal(stdout.Bytes(), &i); err != nil {
		return fmt.Errorf("unmarshaling the output of Azure CLI: %v", err)
	}

	return nil
}
