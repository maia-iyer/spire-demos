#!/bin/sh

# Kubernetes API endpoint
KUBE_API_ENDPOINT="https://kubernetes.default.svc"

# Service account token
TOKEN_PATH="/var/run/secrets/kubernetes.io/serviceaccount/token"
CACERT_PATH="/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
TOKEN=$(cat $TOKEN_PATH)

NAMESPACE="demo"

# Function to update or create a secret
update_or_create_secret() {
    local SECRET_NAME="$1"
    local SECRET_DATA="$2"

    # Check if the secret exists
    local response=$(curl -o /dev/null -w "%{http_code}" -s -X GET \
        -H "Authorization: Bearer $TOKEN" --cacert "$CACERT_PATH" \
        "$KUBE_API_ENDPOINT/api/v1/namespaces/$NAMESPACE/secrets/$SECRET_NAME")

    if [ $response -eq 200 ]; then
        # Secret exists, update it
        curl -o /dev/null -s -X PUT \
            -H "Authorization: Bearer $TOKEN" --cacert "$CACERT_PATH" \
            -H "Content-Type: application/json" \
            -d '{"apiVersion":"v1","kind":"Secret","metadata":{"name":"'${SECRET_NAME}'","namespace":"'${NAMESPACE}'"},"data":{"'${SECRET_NAME}'":"'${SECRET_DATA}'"}}' \
            "$KUBE_API_ENDPOINT/api/v1/namespaces/$NAMESPACE/secrets/$SECRET_NAME"
	echo "Secret ${SECRET_NAME} updated successfully"
    else
        # Secret doesn't exist, create it
        curl -o /dev/null -s -X POST \
            -H "Authorization: Bearer $TOKEN" --cacert "$CACERT_PATH" \
            -H "Content-Type: application/json" \
            -d '{"apiVersion":"v1","kind":"Secret","metadata":{"name":"'${SECRET_NAME}'","namespace":"'${NAMESPACE}'"},"data":{"'${SECRET_NAME}'":"'${SECRET_DATA}'"}}' \
            "$KUBE_API_ENDPOINT/api/v1/namespaces/$NAMESPACE/secrets"
	echo "Secret ${SECRET_NAME} created successfully"
    fi
}

SVID_SECRET_NAME=svid.pem
SVID_SECRET_CONTENT=$(base64 -w 0 svid.pem)

update_or_create_secret "$SVID_SECRET_NAME" "$SVID_SECRET_CONTENT"

SVID_KEY_SECRET_NAME=svidkey.pem
SVID_KEY_SECRET_CONTENT=$(base64 -w 0 svid_key.pem)

update_or_create_secret "$SVID_KEY_SECRET_NAME" "$SVID_KEY_SECRET_CONTENT"

SVID_BUNDLE_SECRET_NAME=svidbundle.pem
SVID_BUNDLE_SECRET_CONTENT=$(base64 -w 0 svid_bundle.pem)

update_or_create_secret "$SVID_BUNDLE_SECRET_NAME" "$SVID_BUNDLE_SECRET_CONTENT"



