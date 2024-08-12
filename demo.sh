IMAGE_TAG=fleetkubecondemo.azurecr.io/kueue:experimental PLATFORMS=linux/amd64 make image-local-push

export IMAGE_REPO=fleetkubecondemo.azurecr.io/kueue
export IMAGE_TAG=experimental
docker push $IMAGE_REPO:$IMAGE_TAG

kubectl config use-context bravelion-admin
helm install kueue ./charts/kueue/ --create-namespace --namespace kueue-system --set controllerManager.manager.image.repository=$IMAGE_REPO --set controllerManager.manager.image.tag=$IMAGE_TAG
VERSION=v0.5.2 
kubectl apply --server-side -f https://github.com/kubernetes-sigs/jobset/releases/download/$VERSION/manifests.yaml

kubectl config use-context hub-admin
helm install kueue ./charts/kueue/ --create-namespace --namespace kueue-system --set controllerManager.manager.image.repository=$IMAGE_REPO --set controllerManager.manager.image.tag=$IMAGE_TAG --set featureGates='MultiKueue=true'
VERSION=v0.5.2
kubectl apply --server-side -f https://github.com/kubernetes-sigs/jobset/releases/download/$VERSION/manifests.yaml

kubectl config use-context bravelion-admin
chmod +x worker-sample.sh 
./worker-sample.sh

kubectl config use-context hub-admin
kubectl create secret generic bravelion -n kueue-system --from-file=kubeconfig=kubeconfig
kubectl apply -f examples/kueue

export REGISTRY=fleetkubecondemo.azurecr.io
export HUB_AGENT_IMAGE=hub-agent
export TAG=experimental
helm install hub-agent ../../charts/hub-agent/ \
    --set image.pullPolicy=Always \
    --set image.repository=$REGISTRY/$HUB_AGENT_IMAGE \
    --set image.tag=$TAG \
    --set namespace=fleet-system \
    --set logVerbosity=5 \
    --set enableWebhook=false \
    --set webhookClientConnectionType=service \
    --set logFileMaxSize=1000000

kubectl create serviceaccount bravelion -n fleet-system
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Secret
metadata:
    name: bravelion
    namespace: fleet-system
    annotations:
        kubernetes.io/service-account.name: bravelion
type: kubernetes.io/service-account-token
EOF

TOKEN=$(kubectl get secret bravelion -n fleet-system -o jsonpath='{.data.token}' | base64 -d)

kubectl config use-context bravelion-admin
kubectl delete secret hub-kubeconfig-secret --ignore-not-found
kubectl create secret generic hub-kubeconfig-secret --from-literal=token=$TOKEN

HUB_SERVER_URL=https://hub-dns-5ivj4uhd.hcp.eastus.azmk8s.io:443
MEMBER_SERVER_URL=https://bravelion-dns-hgyfa403.hcp.eastus.azmk8s.io:443

export MEMBER_AGENT_IMAGE=member-agent
export REFRESH_TOKEN_IMAGE=refresh-token
export PROPERTY_PROVIDER=azure

helm install member-agent ../../charts/member-agent/ \
    --set config.hubURL=$HUB_SERVER_URL \
    --set config.memberURL=$MEMBER_SERVER_URL \
    --set image.repository=$REGISTRY/$MEMBER_AGENT_IMAGE \
    --set image.tag=$TAG \
    --set refreshtoken.repository=$REGISTRY/$REFRESH_TOKEN_IMAGE \
    --set refreshtoken.tag=$TAG \
    --set image.pullPolicy=Always \
    --set refreshtoken.pullPolicy=Always \
    --set config.memberClusterName=bravelion \
    --set logVerbosity=5 \
    --set namespace=fleet-system \
    --set enableV1Alpha1APIs=false \
    --set enableV1Beta1APIs=true \
    --set propertyProvider=$PROPERTY_PROVIDER

kubectl config use-context hub-admin
cat <<EOF | kubectl apply -f -
apiVersion: cluster.kubernetes-fleet.io/v1beta1
kind: MemberCluster
metadata:
  name: bravelion
spec:
  identity:
    name: bravelion
    kind: ServiceAccount
    namespace: fleet-system
    apiGroup: ""
EOF