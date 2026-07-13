#!/bin/bash
rm -r kubevirt-tekton-tasks || true
gh repo clone kubevirt/kubevirt-tekton-tasks
cd kubevirt-tekton-tasks || exit 1

git fetch origin

if [[ -z ${RELEASE_IMAGE_DIGEST} ]]; then
    echo "image digest not defined"
    exit 1
fi
if [[ -z ${RELEASE_IMAGE_NAME} ]]; then
    echo "image name not defined"
    exit 1
fi
upstream_commit=$(skopeo inspect "docker://registry.redhat.io/container-native-virtualization/${RELEASE_IMAGE_NAME}@${RELEASE_IMAGE_DIGEST}" | jq -r '.Labels."upstream-vcs-ref"')
if [[ -z ${upstream_commit} || ${upstream_commit} == "null" ]]; then
    echo "image does not contain an upstream-vcs-ref label, cannot determine upstream commit"
    exit 1
fi
git checkout "${upstream_commit}" || exit 1

cp -r "../ansible/." "scripts/ansible/"

find configs/*.yaml -type f -print0 | xargs -0 sed -i "s/quay.io\/kubevirt\/tekton-tasks-disk-virt/registry.redhat.io\/container-native-virtualization\/kubevirt-tekton-tasks-disk-virt-customize-rhel9/g"
find configs/*.yaml -type f -print0 | xargs -0 sed -i "s/quay.io\/kubevirt\/tekton-tasks/registry.redhat.io\/container-native-virtualization\/${RELEASE_IMAGE_NAME}/g"

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
