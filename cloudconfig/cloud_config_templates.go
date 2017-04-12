package cloudconfig

const (
	BaseCloudConfig = `---
azs: []

compilation:
  az: z1
  network: private
  reuse_compilation_vms: true
  vm_type: default
  vm_extensions:
  - 100GB_ephemeral_disk
  workers: 6

disk_types:
- name: 1GB
  disk_size: 1024
- name: 5GB
  disk_size: 5120
- name: 10GB
  disk_size: 10240
- name: 50GB
  disk_size: 51200
- name: 100GB
  disk_size: 102400
- name: 500GB
  disk_size: 512000
- name: 1TB
  disk_size: 1048576

networks: []

vm_types:
- name: default
- name: sharedcpu
- name: small
- name: medium
- name: large
- name: extra-large

vm_extensions:
- name: 1GB_ephemeral_disk
- name: 5GB_ephemeral_disk
- name: 10GB_ephemeral_disk
- name: 50GB_ephemeral_disk
- name: 100GB_ephemeral_disk
- name: 500GB_ephemeral_disk
- name: 1TB_ephemeral_disk
`
)
