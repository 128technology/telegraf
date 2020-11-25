echo "VERSION_TAG=$VERSION_TAG"

VERSION_REGEX='^128tech-v([0-9]+\.[0-9]+\.[0-9]+)(-([0-9]+))?$'
[[ $VERSION_TAG =~ $VERSION_REGEX ]]

if [ -z $BASH_REMATCH ]; then
    echo "The tagged version does not match the required expression: $VERSION_EXPRESION"
    exit 1
fi

echo "VERSION=${BASH_REMATCH[1]}" >> $GITHUB_ENV

if [ ! -z ${BASH_REMATCH[3]} ]; then
    echo "VERSION_PATCH=${BASH_REMATCH[3]}" >> $GITHUB_ENV
else
    echo "VERSION_PATCH=1" >> $GITHUB_ENV
fi
