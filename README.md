# OVHcloud VPS Pulumi Project

> **WARNING:** Stack configuration files (`Pulumi.*.yaml`) contain encrypted
> credentials and are **not** committed — `.gitignore` excludes them. Pushing
> this repository does **not** publish your OVH credentials or SSH keys, but
> never add a `Pulumi.*.yaml` file to git yourself, and keep the Pulumi
> encryption passphrase out of any shell history or CI logs.

This Pulumi Go project orders an OVHcloud **VPS-1 2027** (`vps-2027-model1`) with the required standard automated backup and local storage options plus the **50 GB additional disk** option (`option-additional-disk-2027-50g`). A normal `pulumi up` installs Ubuntu 26.04, connects over SSH, formats the extra disk as ext4, adds it to `/etc/fstab`, and mounts it at `/mnt/disk`.

The disk is separate from the VPS system disk and survives VPS reinstallation. The guest-side mount command assumes a Linux image.

## Prerequisites

- Go 1.25 or newer
- Pulumi CLI (v3.203.0 or newer)
- An OVHcloud API application with permission to order and manage VPS resources
- A default OVHcloud payment method
- An SSH key pair

The `vps-2027-model1` plan, its mandatory options, the `option-additional-disk-2027-50g` option, and the Ubuntu 26.04 image are OVH 2027-era catalog items. Older or newer OVH catalogs may use different plan codes and image UUIDs; adjust the corresponding Pulumi configuration values if the order is rejected.

## Pricing

The following are published France prices, excluding VAT, checked on 11 August 2026. Prices can change and the final OVH cart is authoritative.

- **VPS-1 2027 (`vps-2027-model1`)** — 2 vCores, 4 GB RAM, and 40 GB NVMe from **€3.81/month** (€4.57/month including VAT). This VPS is billed monthly; OVH does not offer hourly or daily billing for VPS products.
- **Public Cloud B3-8** — 2 vCores, 8 GB RAM, and 50 GB NVMe at **€0.0512/hour** in GRA, or approximately **€1.23/day** and **€36.86 per 30-day month**. Public Cloud instances support hourly billing, with each commenced hour charged. The published one-month Savings Plan price is **€28.03/month**.
- The hourly Public Cloud option becomes more expensive than the base VPS after approximately **74 hours (three days)** of use. It is a separate OVH product and is not selected by this Pulumi program.
- The standard daily backup is included in the published VPS price. The repository also orders a separate 50 GB additional disk; its current public price is not listed and must be confirmed in the OVH cart.

Pricing sources:

- https://www.ovhcloud.com/fr/vps/
- https://www.ovhcloud.com/fr/public-cloud/prices/
- https://docs.ovhcloud.com/en/guides/account-and-service-management/managing-billing-payments-and-services/changing-hourly-monthly-billing

The OVH API application must allow the VPS order operations plus `GET /vps` and `GET /vps/*`. The consumer key must also allow cart assignment and checkout, account payment/order reads and payment, and VPS update/rebuild operations. Full API access for the application key is the simplest setup; otherwise grant the corresponding `order:apiovh:*`, `account:apiovh:*`, and `vps:apiovh:*` IAM actions.

Set OVHcloud credentials in the environment. For application-key authentication:

```bash
export OVH_ENDPOINT=ovh-eu
export OVH_APPLICATION_KEY=...
export OVH_APPLICATION_SECRET=...
export OVH_CONSUMER_KEY=...
```

The provider also supports `OVH_ACCESS_TOKEN` and OAuth2 credentials. See the OVH Pulumi provider documentation for the available authentication methods.

## Configure And Deploy

Create a local stack (use any name you like, e.g. `dev` or `vps-1-2027-dev`) and set the configuration. Pulumi encrypts secrets with a passphrase you choose on first use:

```bash
pulumi stack init vps-1-2027-dev

pulumi config set vps:ovhSubsidiary FR
pulumi config set vps:region europe
pulumi config set vps:vpsDatacenter GRA
pulumi config set vps:vpsOs "Ubuntu 26.04"
pulumi config set vps:publicSshKey "$(cat ~/.ssh/id_ed25519.pub)"
pulumi config set vps:sshUser ubuntu
pulumi config set --secret vps:sshPrivateKey "$(cat ~/.ssh/id_ed25519)"
# Only set this for an encrypted private key.
# pulumi config set --secret vps:sshPrivateKeyPassword "<ssh-key-passphrase>"
# Never commit the passphrase or any Pulumi.*.yaml file.

pulumi plugin install resource ovh v2.17.0 --server github://api.github.com/ovh/pulumi-ovh
pulumi plugin install resource command v1.2.1 --server github://api.github.com/pulumi/pulumi-command

go mod tidy
pulumi preview
pulumi up
```

Without these `vps:` settings the program uses defaults for the region, datacenter, OS, plan, options, and Ubuntu image UUID, but `vps:publicSshKey` and `vps:sshPrivateKey` are required.

The program defaults `vps:vpsImageId` to the verified Ubuntu 26.04 image UUID `7bd8c398-59e6-4322-bc40-0f5021c02afc`. Override it only when selecting another image:

```bash
pulumi config set vps:vpsImageId "<image-uuid>"
```

## How The Image UUID Was Found

OVH exposes image names through the per-image endpoint. The process used for Ubuntu 26.04 was:

1. Query `GET /vps/<serviceName>/images/available` to get candidate image UUIDs.
2. Query `GET /vps/<serviceName>/images/available/<imageUuid>` for each candidate.
3. Select the response whose `name` is `Ubuntu 26.04`; it returned `7bd8c398-59e6-4322-bc40-0f5021c02afc`.

## Why The SSH Private Key Is Required (No Cloud-Init)

`vps:sshPrivateKey` is stored as a Pulumi secret and used **only locally** by Pulumi to SSH into the new VPS as `ubuntu` and run the disk provisioning (partition, format ext4, fstab entry, mount at `/mnt/disk`). It is never sent to OVH; OVH only receives `vps:publicSshKey`, which it installs for the `ubuntu` user.

The OVH VPS API and the `ovh_vps` resource expose **no `userData` / cloud-init / config-drive input** — the only creation-time customization is the public SSH key. Because the 50 GB additional disk is attached raw and blank, it must be partitioned and formatted inside Ubuntu, which requires an authenticated SSH session. There is therefore no supported way to auto-provision the disk without the private key today.

If you want to remove the SSH step, realistic alternatives are:

- **One-time manual setup** — run the equivalent commands over SSH once, then the disk stays mounted across reboots via `/etc/fstab`.
- **Preconfigured custom image** — if OVH supports deploying a custom image with the setup baked in for this VPS plan.
- **Different OVH product** — a Public Cloud instance supports user-data/cloud-init natively.

If the provider reports an error after OVH has created the VPS, do not run the paid order again. Adopt the existing service instead:

```bash
pulumi config set vps:adoptExistingService true
pulumi config set vps:existingServiceName "<created-service-name>"
pulumi up
pulumi config rm vps:adoptExistingService
```

`vps:vpsDatacenter` must be a datacenter offered by the selected VPS plan. For a Canadian deployment, use `vps:region=canada` and a Canadian datacenter such as `BHS` when available.

The optional `vps:sshHostKey` setting enables SSH host-key verification for the mount command. If it is not set, the command provider does not verify the host key.

Useful outputs after deployment:

```bash
pulumi stack output vpsServiceName
pulumi stack output vpsPublicIp
pulumi stack output additionalDiskMountPath
```

Verify the disk is mounted and persistent:

```bash
ssh -i ~/.ssh/id_ed25519 ubuntu@<vpsPublicIp>
sudo findmnt -no SOURCE,FSTYPE,TARGET /mnt/disk
sudo blkid -s UUID -o value /dev/sdb1
sudo grep -E '^[^#].* /mnt/disk ext4 ' /etc/fstab
```

The Pulumi program does not run `pulumi up` or create infrastructure automatically. Review the order and pricing in `pulumi preview` before applying it.

## Development

```bash
go test ./...
go vet ./...
```

## License

MIT — see [LICENSE](LICENSE).
