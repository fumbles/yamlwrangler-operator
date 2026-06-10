Safe Testing Workflow
# 1. Backup everything
./backup-configmaps.sh

# 2. Test the Docker Hub images
oc delete deployment app-dashboard-operator -n app-dashboard-operator
oc apply -f ~/git/yamlwrangler-operator/manifests/deploy/deployment.yaml

helm upgrade app-dashboard charts/openshift-console-plugin --namespace app-dashboard-plugin

# 3. Verify everything works
# (Your ConfigMaps are untouched!)

# 4. If something goes wrong (unlikely):
./restore-configmaps.sh configmap-backups/<timestamp>

Copy to Separate Repo
# Copy scripts to dashboard repo
cp app-dashboard-console-plugin/backup-configmaps.sh ~/git/yamlwrangler-dashboard/
cp app-dashboard-console-plugin/restore-configmaps.sh ~/git/yamlwrangler-dashboard/

# Add to git
cd ~/git/yamlwrangler-dashboard
git add backup-configmaps.sh restore-configmaps.sh
git commit -m "Add ConfigMap backup and restore scripts"
git push
