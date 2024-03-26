#!/bin/sh

BASE_YAML=$1
BUNDLE_FILE=$2
RESULT_FILE=$3

if [ -f "$RESULT_FILE" ]; then
	rm -r $RESULT_FILE
fi

cat $BASE_YAML >> $RESULT_FILE

while IFS= read -r line; do
	echo "    $line" >> $RESULT_FILE
done < $BUNDLE_FILE

