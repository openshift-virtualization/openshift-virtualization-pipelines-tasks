#!/bin/bash
rm -r kubevirt-tekton-tasks
git clone git@github.com:kubevirt/kubevirt-tekton-tasks.git
cd kubevirt-tekton-tasks || exit 1

git fetch origin
git checkout "${RELEASE_BRANCH}"

cp -r "../ansible/." "scripts/ansible/"

find configs/*.yaml -type f -print0 | xargs -0 sed -i "s/quay.io\/kubevirt\/tekton-tasks/registry.access.redhat.com\/container-native-virtualization\/kubevirt-tekton-tasks-create-datavolume-rhel9/g"
find configs/*.yaml -type f -print0 | xargs -0 sed -i "s/quay.io\/kubevirt\/tekton-tasks-disk-virt/registry.access.redhat.com\/container-native-virtualization\/kubevirt-tekton-tasks-disk-virt-customize-rhel9/g"

make generate-yaml-tasks
#make generate-pipelines
#delete tasks, which are not published
for TASK_NAME in "execute-in-vm" "generate-ssh-keys"
do
	rm -r "tasks/${TASK_NAME}"
done
#delete pipelines, which are not published
rm -r "pipelines/windows-bios-installer" "pipelines/windows/customize"

../run-catalog-cd.sh
