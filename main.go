package main

import (
	"fmt"
	"strings"

	"github.com/ovh/pulumi-ovh/sdk/v2/go/ovh/vps"
	"github.com/pulumi/pulumi-command/sdk/go/command/remote"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

const (
	defaultVPSPlanCode         = "vps-2027-model1"
	defaultUbuntuImageID       = "7bd8c398-59e6-4322-bc40-0f5021c02afc"
	defaultAutomatedBackupCode = "option-auto-backup-2027-1-model1"
	defaultStorageCode         = "option-storage-local-2027-model1"
	defaultDiskPlanCode        = "option-additional-disk-2027-50g"
	defaultPlanDuration        = "P1M"
	defaultPricingMode         = "default"
	defaultOVHSubsidiary       = "FR"
	defaultRegion              = "europe"
	defaultDatacenter          = "GRA"
	defaultOperatingSystem     = "Ubuntu 26.04"
	defaultSSHUser             = "ubuntu"
	defaultMountPath           = "/mnt/disk"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cfg := config.New(ctx, "vps")

		ovhSubsidiary := cfg.Get("ovhSubsidiary")
		if ovhSubsidiary == "" {
			ovhSubsidiary = defaultOVHSubsidiary
		}

		region := cfg.Get("region")
		if region == "" {
			region = defaultRegion
		}
		if region != "europe" && region != "canada" {
			return fmt.Errorf("ovh:region must be europe or canada, got %q", region)
		}

		datacenter := cfg.Get("vpsDatacenter")
		if datacenter == "" {
			datacenter = defaultDatacenter
		}

		operatingSystem := cfg.Get("vpsOs")
		if operatingSystem == "" {
			operatingSystem = defaultOperatingSystem
		}

		planCode := cfg.Get("vpsPlanCode")
		if planCode == "" {
			planCode = defaultVPSPlanCode
		}

		diskPlanCode := cfg.Get("additionalDiskPlanCode")
		if diskPlanCode == "" {
			diskPlanCode = defaultDiskPlanCode
		}

		planDuration := cfg.Get("planDuration")
		if planDuration == "" {
			planDuration = defaultPlanDuration
		}

		pricingMode := cfg.Get("pricingMode")
		if pricingMode == "" {
			pricingMode = defaultPricingMode
		}

		mountPath := cfg.Get("mountPath")
		if mountPath == "" {
			mountPath = defaultMountPath
		}
		if err := validateMountPath(mountPath); err != nil {
			return err
		}

		publicSSHKey := cfg.Require("publicSshKey")
		imageID := cfg.Get("vpsImageId")
		if imageID == "" {
			imageID = defaultUbuntuImageID
		}
		existingServiceName := cfg.Get("existingServiceName")
		adoptExistingService := cfg.Get("adoptExistingService") == "true"
		privateSSHKey := cfg.RequireSecret("sshPrivateKey")
		var privateSSHKeyPassword pulumi.StringPtrInput
		if cfg.Get("sshPrivateKeyPassword") != "" {
			privateSSHKeyPassword = cfg.GetSecret("sshPrivateKeyPassword")
		}

		sshUser := cfg.Get("sshUser")
		if sshUser == "" {
			sshUser = defaultSSHUser
		}

		var imageInput pulumi.StringPtrInput
		var publicSSHKeyInput pulumi.StringPtrInput
		imageInput = pulumi.String(imageID)
		publicSSHKeyInput = pulumi.String(publicSSHKey)

		args := &vps.VpsArgs{
			DisplayName:       pulumi.String("vps-1-2027"),
			DoNotSendPassword: pulumi.Bool(true),
			ImageId:           imageInput,
			PublicSshKey:      publicSSHKeyInput,
			OvhSubsidiary:     pulumi.String(ovhSubsidiary),
			Plans: vps.VpsPlanArray{
				&vps.VpsPlanArgs{
					Duration:    pulumi.String(planDuration),
					PlanCode:    pulumi.String(planCode),
					PricingMode: pulumi.String(pricingMode),
					Quantity:    pulumi.Float64(1),
					Configurations: vps.VpsPlanConfigurationArray{
						&vps.VpsPlanConfigurationArgs{
							Label: pulumi.String("region"),
							Value: pulumi.String(region),
						},
						&vps.VpsPlanConfigurationArgs{
							Label: pulumi.String("vps_datacenter"),
							Value: pulumi.String(datacenter),
						},
						&vps.VpsPlanConfigurationArgs{
							Label: pulumi.String("vps_os"),
							Value: pulumi.String(operatingSystem),
						},
					},
				},
			},
			PlanOptions: vps.VpsPlanOptionArray{
				vpsPlanOption(defaultAutomatedBackupCode, planDuration, pricingMode),
				vpsPlanOption(defaultStorageCode, planDuration, pricingMode),
				vpsPlanOption(diskPlanCode, planDuration, pricingMode),
			},
		}
		resourceOptions := []pulumi.ResourceOption(nil)
		if existingServiceName != "" {
			args.Name = pulumi.String(existingServiceName)
			args.Plans = nil
			args.PlanOptions = nil
			args.OvhSubsidiary = nil
			args.DoNotSendPassword = nil
			if adoptExistingService {
				resourceOptions = append(resourceOptions, pulumi.Import(pulumi.ID(existingServiceName)))
			}
		}
		server, err := vps.NewVps(ctx, "vps", args, resourceOptions...)
		if err != nil {
			return err
		}

		vpsInfo := vps.LookupVpsOutput(ctx, vps.LookupVpsOutputArgs{
			ServiceName: server.ServiceName,
		})
		publicIP := vpsInfo.Ips().ApplyT(func(ips []string) string {
			for _, ip := range ips {
				if !strings.Contains(ip, ":") {
					return ip
				}
			}
			if len(ips) > 0 {
				return ips[0]
			}
			return ""
		}).(pulumi.StringOutput)

		connection := &remote.ConnectionArgs{
			Host:               publicIP,
			User:               pulumi.String(sshUser),
			PrivateKey:         privateSSHKey,
			PrivateKeyPassword: privateSSHKeyPassword,
			DialErrorLimit:     pulumi.Int(30),
			PerDialTimeout:     pulumi.Int(30),
		}
		if hostKey := cfg.Get("sshHostKey"); hostKey != "" {
			connection.HostKey = pulumi.String(hostKey)
		}

		mount, err := remote.NewCommand(ctx, "mountAdditionalDisk", &remote.CommandArgs{
			Create:     pulumi.String(additionalDiskSetupScript(mountPath)),
			Connection: connection,
			Logging:    remote.LoggingStdoutAndStderr,
			Triggers: pulumi.Array{
				pulumi.String(diskPlanCode),
				pulumi.String(mountPath),
			},
		}, pulumi.DependsOn([]pulumi.Resource{server}), pulumi.IgnoreChanges([]string{"connection"}))
		if err != nil {
			return err
		}
		mountStatus := mount.Stdout

		ctx.Export("vpsServiceName", server.ServiceName)
		ctx.Export("vpsPublicIp", publicIP)
		ctx.Export("additionalDiskSize", pulumi.String("50 GB"))
		ctx.Export("additionalDiskMountPath", pulumi.String(mountPath))
		ctx.Export("additionalDiskMountStatus", mountStatus)
		return nil
	})
}

func vpsPlanOption(planCode, duration, pricingMode string) *vps.VpsPlanOptionArgs {
	return &vps.VpsPlanOptionArgs{
		Duration:    pulumi.String(duration),
		PlanCode:    pulumi.String(planCode),
		PricingMode: pulumi.String(pricingMode),
		Quantity:    pulumi.Float64(1),
	}
}

func validateMountPath(path string) error {
	if path == "" || !strings.HasPrefix(path, "/") {
		return fmt.Errorf("ovh:mountPath must be an absolute path")
	}
	if strings.ContainsAny(path, " \t\r\n\"'`$\\") {
		return fmt.Errorf("ovh:mountPath contains unsupported shell characters")
	}
	return nil
}

func additionalDiskSetupScript(mountPath string) string {
	script := fmt.Sprintf(`set -eu

mount_path=%q
root_source="$(findmnt -no SOURCE /)"
root_disk_name="$(lsblk -no PKNAME "$root_source" || true)"
if [ -n "$root_disk_name" ]; then
  root_disk="/dev/$root_disk_name"
else
  root_disk="$root_source"
fi

data_device=""
for attempt in $(seq 1 60); do
  data_device="$(lsblk -dnpo NAME,TYPE | awk -v root="$root_disk" '$2 == "disk" && $1 != root { print $1; exit }')"
	if [ -n "$data_device" ]; then
	  break
	fi
	sleep 30
done

if [ -z "$data_device" ]; then
  echo "No additional disk was detected after 30 minutes" >&2
  exit 1
fi

case "$data_device" in
  /dev/nvme*|/dev/mmcblk*) partition="${data_device}p1" ;;
  *) partition="${data_device}1" ;;
esac

if [ ! -b "$partition" ]; then
  printf 'n\np\n1\n\n\nw\n' | fdisk "$data_device"
  partprobe "$data_device" || true
  udevadm settle || true
fi

filesystem_type="$(blkid -s TYPE -o value "$partition" || true)"
if [ "$filesystem_type" != "ext4" ]; then
  mkfs.ext4 -F "$partition"
fi

mkdir -p "$mount_path"
disk_uuid="$(blkid -s UUID -o value "$partition" || true)"
if [ -z "$disk_uuid" ]; then
  echo "No filesystem UUID was found for $partition" >&2
  exit 1
fi
if ! awk -v uuid="$disk_uuid" -v target="$mount_path" '$1 == "UUID=" uuid && $2 == target { found = 1 } END { exit !found }' /etc/fstab; then
  awk -v target="$mount_path" '$2 != target' /etc/fstab > /etc/fstab.tmp
  printf 'UUID=%%s %%s ext4 defaults,nofail 0 2\n' "$disk_uuid" "$mount_path" >> /etc/fstab.tmp
  mv /etc/fstab.tmp /etc/fstab
fi

if mountpoint -q "$mount_path" && [ "$(findmnt -no SOURCE "$mount_path")" != "$partition" ]; then
  umount "$mount_path"
fi
if ! mountpoint -q "$mount_path"; then
  mount "$mount_path"
fi

printf 'Mounted %%s at %%s\n' "$partition" "$mount_path"
`, mountPath)
	return fmt.Sprintf("set -eu\nsudo -n sh -c %s\n", shellQuote(script))
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
