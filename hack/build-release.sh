# !/bin/bash

# Note that this does not actually meet the full semver spec at all, but
# rather just works for tags that we currently expect. It also matches more
# than we want thanks to the *s involved, so it's more of just a sanity check.
# Using bash extended globbing is not possible with git.
SEMVER_GLOB_PATTERN="v[0-9]*\.[0-9]*\.[0-9]*"
GIT_DESCRIBE_CMD="git describe --dirty"
GIT_DESCRIBE_EXACT_MATCH_CMD="$GIT_DESCRIBE_CMD --exact-match --match=$SEMVER_GLOB_PATTERN"

tag=$($GIT_DESCRIBE_EXACT_MATCH_CMD)
if [[ "$tag" == "" || "$tag" =~ .*dirty ]]; then
    >&2 echo "ERROR: Must be on an annotated tag matching semver \
glob pattern $SEMVER_GLOB_PATTERN with a clean index and working tree."
    >&2 echo "You're on '$($GIT_DESCRIBE_CMD)'."
    exit 1
fi

echo "Verified that we're on a semver tag: $tag"

echo "Building docker image with tag $tag"
make IMAGE_TAG=$tag build

exit 0
