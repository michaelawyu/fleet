if [ -z "${REGISTRY}" ]; then
  echo "REGISTRY is not set."
  exit 1
fi
export HUB_AGENT_IMAGE=hub-agent
export MEMBER_AGENT_IMAGE=member-agent
export REFRESH_TOKEN_IMAGE=refresh-token
export TAG=experimental
if [ -z "${HUB_SERVER_URL}" ]; then
  echo "HUB_SERVER_URL is not set."
  exit 1
fi

az acr login -n $REGISTRY

make -C "../" docker-build-hub-agent
make -C "../" docker-build-member-agent
make -C "../" docker-build-refresh-token

docker push $REGISTRY/$HUB_AGENT_IMAGE:$TAG
docker push $REGISTRY/$MEMBER_AGENT_IMAGE:$TAG
docker push $REGISTRY/$REFRESH_TOKEN_IMAGE:$TAG

helm install hub-agent ../charts/hub-agent/ \
    --set image.pullPolicy=Always \
    --set image.repository=$REGISTRY/$HUB_AGENT_IMAGE \
    --set image.tag=$TAG \
    --set namespace=fleet-system \
    --set logVerbosity=5 \
    --set enableWebhook=false \
    --set webhookClientConnectionType=service \
    --set forceDeleteWaitTime="1m0s" \
    --set clusterUnhealthyThreshold="3m0s" \
    --set logFileMaxSize=1000000

kubectl create serviceaccount fleet-member-agent-1 -n fleet-system
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Secret
metadata:
    name: fleet-member-agent-1-sa
    namespace: fleet-system
    annotations:
        kubernetes.io/service-account.name: fleet-member-agent-1
type: kubernetes.io/service-account-token
EOF

kubectl create serviceaccount fleet-member-agent-2 -n fleet-system
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Secret
metadata:
    name: fleet-member-agent-2-sa
    namespace: fleet-system
    annotations:
        kubernetes.io/service-account.name: fleet-member-agent-2
type: kubernetes.io/service-account-token
EOF

kubectl config use-context hub-admin
TOKEN=$(kubectl get secret fleet-member-agent-1-sa -n fleet-system -o jsonpath='{.data.token}' | base64 -d)
kubectl config use-context member-1-admin
kubectl delete secret hub-kubeconfig-secret --ignore-not-found
kubectl create secret generic hub-kubeconfig-secret --from-literal=token=$TOKEN

kubectl config use-context hub-admin
TOKEN=$(kubectl get secret fleet-member-agent-2-sa -n fleet-system -o jsonpath='{.data.token}' | base64 -d)
kubectl config use-context member-2-admin
kubectl delete secret hub-kubeconfig-secret --ignore-not-found
kubectl create secret generic hub-kubeconfig-secret --from-literal=token=$TOKEN

kubectl config use-context member-1-admin
helm install member-agent ../charts/member-agent/ \
    --set config.hubURL=$HUB_SERVER_URL \
    --set image.repository=$REGISTRY/$MEMBER_AGENT_IMAGE \
    --set image.tag=$TAG \
    --set refreshtoken.repository=$REGISTRY/$REFRESH_TOKEN_IMAGE \
    --set refreshtoken.tag=$TAG \
    --set image.pullPolicy=Always \
    --set refreshtoken.pullPolicy=Always \
    --set config.memberClusterName="member-1" \
    --set logVerbosity=5 \
    --set namespace=fleet-system \
    --set enableV1Alpha1APIs=false \
    --set enableV1Beta1APIs=true \
    --set propertyProvider=azure

kubectl config use-context member-2-admin
helm install member-agent ../charts/member-agent/ \
    --set config.hubURL=$HUB_SERVER_URL \
    --set image.repository=$REGISTRY/$MEMBER_AGENT_IMAGE \
    --set image.tag=$TAG \
    --set refreshtoken.repository=$REGISTRY/$REFRESH_TOKEN_IMAGE \
    --set refreshtoken.tag=$TAG \
    --set image.pullPolicy=Always \
    --set refreshtoken.pullPolicy=Always \
    --set config.memberClusterName="member-2" \
    --set logVerbosity=5 \
    --set namespace=fleet-system \
    --set enableV1Alpha1APIs=false \
    --set enableV1Beta1APIs=true \
    --set propertyProvider=azure

cat <<EOF | kubectl apply -f -
apiVersion: cluster.kubernetes-fleet.io/v1beta1
kind: MemberCluster
metadata:
  name: member-1
spec:
  identity:
    name: fleet-member-agent-1
    kind: ServiceAccount
    namespace: fleet-system
    apiGroup: ""
EOF

cat <<EOF | kubectl apply -f -
apiVersion: cluster.kubernetes-fleet.io/v1beta1
kind: MemberCluster
metadata:
  name: member-2
spec:
  identity:
    name: fleet-member-agent-2
    kind: ServiceAccount
    namespace: fleet-system
    apiGroup: ""
EOF
