package unsupported_test

import (
	"bytes"
	"errors"
	"time"

	"github.com/pivotal-cf-experimental/bosh-bootloader/aws"
	"github.com/pivotal-cf-experimental/bosh-bootloader/aws/cloudformation"
	"github.com/pivotal-cf-experimental/bosh-bootloader/aws/cloudformation/templates"
	"github.com/pivotal-cf-experimental/bosh-bootloader/aws/ec2"
	"github.com/pivotal-cf-experimental/bosh-bootloader/boshinit"
	"github.com/pivotal-cf-experimental/bosh-bootloader/commands"
	"github.com/pivotal-cf-experimental/bosh-bootloader/commands/unsupported"
	"github.com/pivotal-cf-experimental/bosh-bootloader/fakes"
	"github.com/pivotal-cf-experimental/bosh-bootloader/storage"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("DeployBOSHOnAWSForConcourse", func() {
	Describe("Execute", func() {
		var (
			command              unsupported.DeployBOSHOnAWSForConcourse
			stdout               *bytes.Buffer
			builder              *fakes.TemplateBuilder
			stackManager         *fakes.StackManager
			keyPairManager       *fakes.KeyPairManager
			cloudFormationClient *fakes.CloudFormationClient
			clientProvider       *fakes.ClientProvider
			ec2Client            *fakes.EC2Client
			incomingState        storage.State
			globalFlags          commands.GlobalFlags
		)

		BeforeEach(func() {
			stdout = bytes.NewBuffer([]byte{})
			builder = &fakes.TemplateBuilder{}

			cloudFormationClient = &fakes.CloudFormationClient{}
			ec2Client = &fakes.EC2Client{}

			clientProvider = &fakes.ClientProvider{}
			clientProvider.CloudFormationClientCall.Returns.Client = cloudFormationClient
			clientProvider.EC2ClientCall.Returns.Client = ec2Client

			stackManager = &fakes.StackManager{}
			keyPairManager = &fakes.KeyPairManager{}

			logger := &fakes.Logger{}
			command = unsupported.NewDeployBOSHOnAWSForConcourse(builder, stackManager, keyPairManager, clientProvider, boshinit.NewManifestBuilder(logger), stdout)

			builder.BuildCall.Returns.Template = templates.Template{
				AWSTemplateFormatVersion: "some-template-version",
				Description:              "some-description",
				Parameters: map[string]templates.Parameter{
					"KeyName": {
						Type:        "AWS::EC2::KeyPair::KeyName",
						Default:     "some-keypair-name",
						Description: "SSH KeyPair to use for instances",
					},
				},
				Mappings:  map[string]interface{}{},
				Resources: map[string]templates.Resource{},
			}

			globalFlags = commands.GlobalFlags{
				EndpointOverride: "some-endpoint",
			}

			incomingState = storage.State{
				AWS: storage.AWS{
					Region:          "some-aws-region",
					SecretAccessKey: "some-secret-access-key",
					AccessKeyID:     "some-access-key-id",
				},
				KeyPair: &storage.KeyPair{
					Name:       "some-keypair-name",
					PrivateKey: "some-private-key",
					PublicKey:  "some-public-key",
				},
			}

			keyPairManager.SyncCall.Returns.KeyPair = ec2.KeyPair{
				Name:       "some-keypair-name",
				PrivateKey: []byte("some-private-key"),
				PublicKey:  []byte("some-public-key"),
			}
		})

		It("creates/updates the stack with the given name", func() {
			_, err := command.Execute(globalFlags, incomingState)
			Expect(err).NotTo(HaveOccurred())

			Expect(clientProvider.CloudFormationClientCall.Receives.Config).To(Equal(aws.Config{
				AccessKeyID:      "some-access-key-id",
				SecretAccessKey:  "some-secret-access-key",
				Region:           "some-aws-region",
				EndpointOverride: "some-endpoint",
			}))
			Expect(builder.BuildCall.Receives.KeyPairName).To(Equal("some-keypair-name"))
			Expect(stackManager.CreateOrUpdateCall.Receives.Client).To(Equal(cloudFormationClient))
			Expect(stackManager.CreateOrUpdateCall.Receives.StackName).To(Equal("concourse"))
			Expect(stackManager.CreateOrUpdateCall.Receives.Template).To(Equal(templates.Template{
				AWSTemplateFormatVersion: "some-template-version",
				Description:              "some-description",
				Parameters: map[string]templates.Parameter{
					"KeyName": {
						Type:        "AWS::EC2::KeyPair::KeyName",
						Default:     "some-keypair-name",
						Description: "SSH KeyPair to use for instances",
					},
				},
				Mappings:  map[string]interface{}{},
				Resources: map[string]templates.Resource{},
			}))

			Expect(stackManager.WaitForCompletionCall.Receives.Client).To(Equal(cloudFormationClient))
			Expect(stackManager.WaitForCompletionCall.Receives.StackName).To(Equal("concourse"))
			Expect(stackManager.WaitForCompletionCall.Receives.SleepInterval).To(Equal(15 * time.Second))
		})

		It("syncs the keypair", func() {
			state, err := command.Execute(globalFlags, incomingState)
			Expect(err).NotTo(HaveOccurred())

			Expect(clientProvider.EC2ClientCall.Receives.Config).To(Equal(aws.Config{
				AccessKeyID:      "some-access-key-id",
				SecretAccessKey:  "some-secret-access-key",
				Region:           "some-aws-region",
				EndpointOverride: "some-endpoint",
			}))
			Expect(keyPairManager.SyncCall.Receives.EC2Client).To(Equal(ec2Client))
			Expect(keyPairManager.SyncCall.Receives.KeyPair).To(Equal(ec2.KeyPair{
				Name:       "some-keypair-name",
				PrivateKey: []byte("some-private-key"),
				PublicKey:  []byte("some-public-key"),
			}))

			Expect(state.KeyPair).To(Equal(&storage.KeyPair{
				Name:       "some-keypair-name",
				PublicKey:  "some-public-key",
				PrivateKey: "some-private-key",
			}))
		})

		It("returns the given state unmodified", func() {
			_, err := command.Execute(globalFlags, incomingState)
			Expect(err).NotTo(HaveOccurred())
		})

		It("prints out the bosh-init manifest", func() {
			stackManager.DescribeCall.Returns.Output = cloudformation.Stack{
				Outputs: map[string]string{
					"BOSHSubnet":              "subnet-12345",
					"BOSHSubnetAZ":            "some-az",
					"BOSHEIP":                 "some-elastic-ip",
					"BOSHUserAccessKey":       "some-access-key-id",
					"BOSHUserSecretAccessKey": "some-secret-access-key",
				},
			}

			_, err := command.Execute(globalFlags, incomingState)
			Expect(err).NotTo(HaveOccurred())

			Expect(stdout.String()).To(ContainSubstring("bosh-init manifest:"))
			Expect(stdout.String()).To(ContainSubstring("name: bosh"))
			Expect(stdout.String()).To(ContainSubstring("subnet: subnet-12345"))
			Expect(stdout.String()).To(ContainSubstring("availability_zone: some-az"))
			Expect(stdout.String()).To(ContainSubstring("static_ips:\n    - some-elastic-ip"))
			Expect(stdout.String()).To(ContainSubstring("host: some-elastic-ip"))
			Expect(stdout.String()).To(ContainSubstring("mbus: https://mbus:mbus-password@some-elastic-ip:6868"))
			Expect(stdout.String()).To(ContainSubstring("access_key_id: some-access-key-id"))
			Expect(stdout.String()).To(ContainSubstring("secret_access_key: some-secret-access-key"))
			Expect(stdout.String()).To(ContainSubstring("region: some-aws-region"))
			Expect(stdout.String()).To(ContainSubstring("default_key_name: some-keypair-name"))
		})

		Context("when there is no keypair", func() {
			BeforeEach(func() {
				incomingState.KeyPair = nil
			})

			It("syncs with an empty keypair", func() {
				_, err := command.Execute(globalFlags, incomingState)
				Expect(err).NotTo(HaveOccurred())

				Expect(keyPairManager.SyncCall.Receives.EC2Client).To(Equal(ec2Client))
				Expect(keyPairManager.SyncCall.Receives.KeyPair).To(Equal(ec2.KeyPair{
					Name:       "",
					PrivateKey: []byte(""),
					PublicKey:  []byte(""),
				}))
			})
		})

		Context("failure cases", func() {
			It("returns an error when the cloudformation client can not be created", func() {
				clientProvider.CloudFormationClientCall.Returns.Error = errors.New("error creating client")

				_, err := command.Execute(globalFlags, incomingState)
				Expect(err).To(MatchError("error creating client"))
			})

			It("returns an error when the ec2 client can not be created", func() {
				clientProvider.EC2ClientCall.Returns.Error = errors.New("error creating client")

				_, err := command.Execute(globalFlags, incomingState)
				Expect(err).To(MatchError("error creating client"))
			})

			It("returns an error when the key pair fails to sync", func() {
				keyPairManager.SyncCall.Returns.Error = errors.New("error syncing key pair")

				_, err := command.Execute(globalFlags, incomingState)
				Expect(err).To(MatchError("error syncing key pair"))
			})

			It("returns an error when the stack can not be created", func() {
				stackManager.CreateOrUpdateCall.Returns.Error = errors.New("error creating stack")

				_, err := command.Execute(globalFlags, incomingState)
				Expect(err).To(MatchError("error creating stack"))
			})

			It("returns an error when waiting for completion errors", func() {
				stackManager.WaitForCompletionCall.Returns.Error = errors.New("error waiting on stack")

				_, err := command.Execute(globalFlags, incomingState)
				Expect(err).To(MatchError("error waiting on stack"))
			})

			It("returns an error when describe stacks returns an error", func() {
				stackManager.DescribeCall.Returns.Error = errors.New("error describing stack")

				_, err := command.Execute(globalFlags, incomingState)
				Expect(err).To(MatchError("error describing stack"))
			})
		})
	})
})