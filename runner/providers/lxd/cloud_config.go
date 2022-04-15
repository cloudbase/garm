package lxd

var cloudConfigTemplate = `
#cloud-config
package_upgrade: true
packages:
  - curl
ssh_authorized_keys:
	{{ ssh_authorized_keys }}
system_info:
  default_user:
    name: runner
	home: /home/runner
	shell: /bin/bash
	groups: [sudo, plugdev, dip, netdev]
	sudo: ALL=(ALL) NOPASSWD:ALL

runcmd:
 - [ ls, -l, / ]
 - [ sh, -xc, "echo $(date) ': hello world!'" ]
 - [ sh, -c, echo "=========hello world=========" ]
 - ls -l /root
 # Note: Don't write files to /tmp from cloud-init use /run/somedir instead.
 # Early boot environments can race systemd-tmpfiles-clean LP: #1707222.
 - mkdir /run/mydir
 - [ wget, "http://slashdot.org", -O, /run/mydir/index.html ]
`
