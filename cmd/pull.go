package cmd

import (
	"fmt"
	"os"
	"path"

	"cloud.google.com/go/firestore"
	"github.com/AshutoshPatole/ssm/internal/security"
	"github.com/AshutoshPatole/ssm/internal/ssh"
	"github.com/AshutoshPatole/ssm/internal/store"
	"github.com/sirupsen/logrus"

	"context"

	"github.com/spf13/cobra"
)

// pullCmd represents the pull command
var pullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Pull your configurations from the cloud",
	Long:  ``,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Initialize Firebase here
		if err := store.InitFirebase(); err != nil {
			logrus.Fatalln("Failed to initialize Firebase:", err)
		}
	},
	Run: func(cmd *cobra.Command, args []string) {

		if store.App == nil {
			fmt.Println("Firebase app is not initialized. Please run the sync command first.")
			return
		}
		downloadConfigurations()
	},
}

func init() {
	syncCmd.AddCommand(pullCmd)
}

func downloadConfigurations() {
	client, err := store.App.Firestore(context.Background())
	if err != nil {
		logrus.Fatal(err)
	}
	defer func(client *firestore.Client) {
		err := client.Close()
		if err != nil {
			logrus.Fatal(err)
		}
	}(client)

	userPassword, _ := ssh.AskPassword()
	uid := fetchUID(userPassword)

	logrus.Debugf("Fetching user configurations %s", uid)

	document, err := client.Collection("configurations").Doc(uid).Get(context.Background())
	if err != nil {
		logrus.Info("Did not found any configuration for current user")
		logrus.Debugf(err.Error())
		return
	}
	if document.Exists() {
		logrus.Debugf("Found configuration for current user %s", uid)
	}

	userHomeDir, err := os.UserHomeDir()
	if err != nil {
		logrus.Fatal(err)
	}

	yamlEncrypted := document.Data()["ssm_yaml"].(string)
	publicKeyEncrypted := document.Data()["public"].(string)
	privateKeyEncrypted := document.Data()["private"].(string)
	zshrcEncrypted := document.Data()["zshrc"].(string)
	bashrcEncrypted := document.Data()["bashrc"].(string)
	sshConfigEncrypted := document.Data()["ssh_config"].(string)
	tmuxEncrypted := document.Data()["tmux"].(string)

	key := security.GenerateEncryptionKey(userPassword)

	yaml, err := security.DecryptData(yamlEncrypted, key)
	if err != nil {
		logrus.Fatal(err)
	}

	publicKey, err := security.DecryptData(publicKeyEncrypted, key)
	if err != nil {
		logrus.Fatal(err)
	}

	privateKey, err := security.DecryptData(privateKeyEncrypted, key)
	if err != nil {
		logrus.Fatal(err)
	}

	bashrc, err := security.DecryptData(bashrcEncrypted, key)
	if err != nil {
		logrus.Fatal(err)
	}

	zshrc, err := security.DecryptData(zshrcEncrypted, key)
	if err != nil {
		logrus.Fatal(err)
	}

	sshConfig, err := security.DecryptData(sshConfigEncrypted, key)
	if err != nil {
		logrus.Fatal(err)
	}

	tmux, err := security.DecryptData(tmuxEncrypted, key)
	if err != nil {
		logrus.Fatal(err)
	}

	// check if .ssh exists at home
	sshDir := userHomeDir + "/.ssh"
	_, err = os.Stat(sshDir)
	if os.IsNotExist(err) {
		err = os.MkdirAll(sshDir, 0755)
		if err != nil {
			logrus.Fatal(err)
		}
	}
	files := map[string]struct {
		data        []byte
		permissions os.FileMode
	}{
		".ssm.yaml":           {yaml, 0644},
		".ssh/id_ed25519.pub": {publicKey, 0644},
		".ssh/id_ed25519":     {privateKey, 0600},
		".bashrc":             {bashrc, 0644},
		".zshrc":              {zshrc, 0644},
		".ssh/config":         {sshConfig, 0644},
		".tmux.conf":          {tmux, 0644},
	}

	for filePath, fileInfo := range files {
		fullPath := path.Join(userHomeDir, filePath)
		logrus.Infof("Attempting to save file: %s", fullPath)
		logrus.Debugf("File content length: %d", len(fileInfo.data))

		if err := saveFile(fullPath, fileInfo.data, fileInfo.permissions); err != nil {
			logrus.Errorf("Failed to save %s: %v", filePath, err)
		}
	}
}

func saveFile(filename string, data []byte, permission os.FileMode) error {
	err := os.WriteFile(filename, data, permission)
	if err != nil {
		return fmt.Errorf("failed to save file %s: %w", filename, err)
	}
	logrus.Infof("Successfully saved file: %s", filename)
	return nil
}

func fetchUID(userPassword string) string {
	userMap, err := store.LoginUser(userEmail, userPassword)
	if err != nil {
		logrus.Fatal(err)
	}
	userId := userMap["user_id"].(string)
	return userId
}
