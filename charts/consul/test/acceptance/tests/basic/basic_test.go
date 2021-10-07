package basic

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/hashicorp/consul-k8s/charts/consul/test/acceptance/framework/consul"
	"github.com/hashicorp/consul-k8s/charts/consul/test/acceptance/framework/helpers"
	"github.com/hashicorp/consul-k8s/charts/consul/test/acceptance/framework/logger"
	"github.com/hashicorp/consul/api"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Test that the basic installation, i.e. just
// servers and clients, works by creating a kv entry
// and subsequently reading it from Consul.
func TestBasicInstallation(t *testing.T) {
	cases := []struct {
		secure      bool
		autoEncrypt bool
	}{
		{
			false,
			false,
		},
		{
			true,
			false,
		},
		{
			true,
			true,
		},
	}

	for _, c := range cases {
		name := fmt.Sprintf("secure: %t, auto-encrypt: %t", c.secure, c.autoEncrypt)
		t.Run(name, func(t *testing.T) {
			releaseName := helpers.RandomName()
			helmValues := map[string]string{
				"global.acls.manageSystemACLs":         strconv.FormatBool(c.secure),
				"global.tls.enabled":                   strconv.FormatBool(c.secure),
				"global.gossipEncryption.autoGenerate": strconv.FormatBool(c.secure),
				"global.tls.enableAutoEncrypt":         strconv.FormatBool(c.autoEncrypt),
			}
			consulCluster := consul.NewHelmCluster(t, helmValues, suite.Environment().DefaultContext(t), suite.Config(), releaseName)

			consulCluster.Create(t)

			client := consulCluster.SetupConsulClient(t, c.secure)

			// Create a KV entry
			randomKey := helpers.RandomName()
			randomValue := []byte(helpers.RandomName())
			logger.Logf(t, "creating KV entry with key %s", randomKey)
			_, err := client.KV().Put(&api.KVPair{
				Key:   randomKey,
				Value: randomValue,
			}, nil)
			require.NoError(t, err)

			logger.Logf(t, "reading value for key %s", randomKey)
			kv, _, err := client.KV().Get(randomKey, nil)
			require.NoError(t, err)
			require.Equal(t, kv.Value, randomValue)

			// Check that autogenerated gossip encryption key is being used
			if c.secure {
				secretName := fmt.Sprintf("%s-consul-gossip-encryption-key", releaseName)
				secretKey := "key"

				keyring, err := client.Operator().KeyringList(nil)
				require.NoError(t, err)

				testContext := suite.Environment().DefaultContext(t)
				secret, err := testContext.KubernetesClient(t).CoreV1().Secrets(testContext.KubectlOptions(t).Namespace).Get(context.Background(), secretName, metav1.GetOptions{})
				require.NoError(t, err)
				gossipEncryptionKey := strings.TrimSpace(string(secret.Data[secretKey]))

				require.Len(t, keyring, 2)
				require.Contains(t, keyring[0].Keys, gossipEncryptionKey)
				require.Contains(t, keyring[1].Keys, gossipEncryptionKey)
			}
		})
	}
}
