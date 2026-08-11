# Agent Instructions

## Project

This repository provisions the dedicated Pulumi stack `vps-1-2027-dev` in OVHcloud. Do not use the unrelated `dev` stack.

The target is:

- VPS plan: `vps-2027-model1`
- Region: `europe`
- Datacenter: `GRA`
- OS: Ubuntu 26.04
- SSH user: `ubuntu`
- Additional disk: `option-additional-disk-2027-50g`, mounted at `/mnt/disk`
- Mandatory options: `option-auto-backup-2027-1-model1` and `option-storage-local-2027-model1`

## Normal Workflow

Provisioning is intentionally one step. The program has a default Ubuntu image UUID, so after configuring credentials and SSH keys run:

```bash
PULUMI_CONFIG_PASSPHRASE='<local-passphrase>' pulumi up --stack vps-1-2027-dev --yes --non-interactive
```

Do not write the Pulumi passphrase or any OVH credential to this file or to the repository. An encrypted SSH private key may use `vps:sshPrivateKeyPassword`; omit that setting for an unencrypted key.

## Ubuntu Image UUID

The default image UUID is `7bd8c398-59e6-4322-bc40-0f5021c02afc`.

It was obtained from the OVH API after the VPS service existed:

1. `GET /vps/<serviceName>/images/available` returned candidate UUIDs.
2. `GET /vps/<serviceName>/images/available/<imageUuid>` returned the image metadata.
3. The metadata response with `name: "Ubuntu 26.04"` returned:

```json
{"name":"Ubuntu 26.04","id":"7bd8c398-59e6-4322-bc40-0f5021c02afc"}
```

The API calls must be authenticated with the configured OVH application credentials. Never print those credentials while debugging. To select a different image, set `vps:vpsImageId` explicitly.

## Provider Caveats

- OVH requires the automated backup and local storage options for this VPS plan.
- The provider attaches option configuration entries to the parent cart item. Set `region` once on the base plan; do not repeat it on every option.
- If `pulumi up` reports a create error after OVH may have completed the paid order, first inspect `GET /vps` and do not order again blindly.
- To recover an existing service, set `vps:existingServiceName` and `vps:adoptExistingService=true`, run `pulumi up`, then remove `vps:adoptExistingService`.
- The Ubuntu image provisions the configured key for `ubuntu`; disk operations run through passwordless `sudo`.
- The mount script must verify filesystem `TYPE=ext4` and a non-empty filesystem UUID. A partition `PARTUUID` alone is not proof that a filesystem exists.

## Why The Private Key Is Needed

`vps:sshPrivateKey` is a hard requirement (`cfg.RequireSecret`). It is **not** sent to OVH — only `vps:publicSshKey` is (installed by OVH for the `ubuntu` user). The private key is used client-side by Pulumi to open an SSH connection to the freshly created VPS and run the disk-setup script (partition + `mkfs.ext4` + fstab entry + mount at `/mnt/disk`).

There is currently **no way to auto-provision the disk without SSH**:

- The OVH VPS API and `ovh_vps` resource expose **no `userData` / cloud-init / config-drive input**. Only the public SSH key is supported during creation.
- The 50 GB additional disk is attached raw and blank; it must be partitioned/formatted inside Ubuntu.
- `pulumi-command` runs the provisioning as an SSH command, which requires the private key.

If asked to "remove the private key" or "use cloud-init", explain these limitations before making changes. Alternatives are manual one-time disk setup, a preconfigured custom image (if OVH supports it for this plan), or moving to a different OVH product that supports user data. Do not pretend cloud-init works; `ovh.VpsArgs` has no such input.

## Validation

Run these checks after code changes:

```bash
go test ./...
go vet ./...
PULUMI_CONFIG_PASSPHRASE='<local-passphrase>' pulumi preview --stack vps-1-2027-dev --non-interactive
```

For live validation, inspect `vpsPublicIp`, `vpsServiceName`, and `additionalDiskMountStatus`, then verify over SSH as `ubuntu`:

```bash
sudo findmnt -no SOURCE,FSTYPE,TARGET /mnt/disk
sudo blkid -s UUID -o value /dev/sdb1
sudo grep -E '^[^#].* /mnt/disk ext4 ' /etc/fstab
```
