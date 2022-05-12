#cloud-config
package_upgrade: true
packages:
    - curl
    - tar
system_info:
    default_user:
        name: runner
        home: /home/runner
        shell: /bin/bash
        groups:
            - sudo
            - adm
            - cdrom
            - dialout
            - dip
            - video
            - plugdev
            - netdev
        sudo: ALL=(ALL) NOPASSWD:ALL
runcmd:
    - /install_runner.sh
    - rm -f /install_runner.sh
write_files:
    - encoding: b64
      content: RUNNER_INSTALL_B64
      owner: root:root
      path: /install_runner.sh
      permissions: "755"
