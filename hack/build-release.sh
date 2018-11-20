# !/bin/bash

SEMVER_PATTERN="v[0-9]*\.[0-9]*\.[0-9]*"
GIT_DESCRIBE_CMD="git describe --dirty"
GIT_DESCRIBE_EXACT_MATCH_CMD="$GIT_DESCRIBE_CMD --exact-match --match=$SEMVER_PATTERN"

tag=$($GIT_DESCRIBE_EXACT_MATCH_CMD)
if [[ "$tag" == "" || "$tag" =~ .*dirty ]]; then
    >&2 echo "ERROR: Must be on an annotated tag matching semver \
pattern $SEMVER_PATTERN with a clean index and working tree."
    >&2 echo "You're on '$($GIT_DESCRIBE_CMD)'."
    exit 1
fi

echo "Verified that we're on a semver tag: $tag"

echo "Building agent docker image with tag $tag"
make AGENT_IMAGE_TAG=$tag build-agent

echo "Building coordinator docker image with tag $tag"
make COORDINATOR_IMAGE_TAG=$tag build-coordinator

exit 0
