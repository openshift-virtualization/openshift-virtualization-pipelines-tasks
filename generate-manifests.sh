#!/bin/bash
rm -r kubevirt-tekton-tasks || true
gh repo clone kubevirt/kubevirt-tekton-tasks
cd kubevirt-tekton-tasks || exit 1

git fetch origin
git checkout "${RELEASE_BRANCH}"

cp -r "../ansible/." "scripts/ansible/"

find configs/*.yaml -type f -print0 | xargs -0 sed -i "s/quay.io\/kubevirt\/tekton-tasks-disk-virt/registry.redhat.io\/container-native-virtualization\/kubevirt-tekton-tasks-disk-virt-customize-rhel9/g"
find configs/*.yaml -type f -print0 | xargs -0 sed -i "s/quay.io\/kubevirt\/tekton-tasks/registry.redhat.io\/container-native-virtualization\/kubevirt-tekton-tasks-create-datavolume-rhel9/g"

make generate-yaml-tasks
make generate-pipelines

find release/pipelines/*/*.yaml -type f -print0 | xargs -0 sed -i "s/quay.io\/kubevirt\/virtio-container-disk.*/registry.redhat.io\/container-native-virtualization\/virtio-win-rhel9:${RELEASE_VERSION}/g"

#delete tasks, which are not published
for TASK_NAME in "execute-in-vm" "generate-ssh-keys"
do
	rm -r "release/tasks/${TASK_NAME}"
done
#delete pipelines, which are not published
rm -r "release/pipelines/windows-bios-installer" "release/pipelines/windows-customize"

../run-catalog-cd.sh
