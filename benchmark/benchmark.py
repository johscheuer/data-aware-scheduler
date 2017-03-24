#!/usr/bin/env python3

import time
import sys
import subprocess
import yaml

from kubernetes import config
from kubernetes.client import configuration
from kubernetes.client.apis import core_v1_api
from kubernetes.client.apis import batch_v1_api
from kubernetes.client.rest import ApiException


def create_qmgmt_cmd(cmd):
    return [
        '/bin/sh',
        '-c',
        'qmgmt -u api:7860 {}'.format(cmd)]


def cleanup_quobyte_volume(api, volume, config):
    print('Clean up Quobyte Volume. {}'.format(volume))
    rsp = api.connect_get_namespaced_pod_exec(name, namespace,
                                            command=create_qmgmt_cmd(
                                                'volume show {}'.format(volume)),
                                            stderr=True, stdin=False,
                                            stdout=True, tty=False)

    if 'No such volume: {}'.format(volume) not in rsp:
        api.connect_get_namespaced_pod_exec(name, namespace,
                                                   command=create_qmgmt_cmd(
                                                       'volume delete -f {}'.format(volume)),
                                                   stderr=True, stdin=False,
                                                   stdout=True, tty=False)

    print(api.connect_get_namespaced_pod_exec(name, namespace,
                                               command=create_qmgmt_cmd(
                                                   'volume create {} root root {} 0777'.format(volume, config)),
                                               stderr=True, stdin=False,
                                               stdout=True, tty=False))


def create_and_wait_for_completion(batch_api, file_name):
    with open(file_name, 'r', encoding='utf-8') as content:
        body = yaml.safe_load(content)
    if body is None:
        raise ValueError('Body of file {} is empty'.format(file_name))
    # TODO if non data-local set node selector

#    try:
#        api_response = batch_api.create_namespaced_job('default', body)
#    except ApiException as e:
#        print("Exception when calling BatchV1Api->create_namespaced_job: %s\n" % e)

    try:
        api_response = batch_api.read_namespaced_job_status(body['metadata']['name'], 'default')
        while api_response is not None and api_response['status']['succeeded'] is None:
            time.sleep(45)
    except ApiException as e:
        print("Exception when calling BatchV1Api->read_namespaced_job: %s\n" % e)


"""
function wait_for_completion() {
    max_waits=180
    waits=0
    until $(kubectl get job -l job-name=$1 --no-headers -o json | jq '.items[0].status.succeeded == 1'); do
    if [[ waits -ge max_waits ]]; then
        echo "Max wait... break up"
        break
    fi

    echo "Wait for $1 to finish"
    sleep 60
    let waits++
    done;
}
"""
config.load_kube_config()
api = core_v1_api.CoreV1Api()
name = 'qmgmt-pod'
namespace = 'quobyte'
resp = None
try:
    resp = api.read_namespaced_pod(name=name,
                                   namespace=namespace)
except ApiException as e:
    if e.status != 404:
        print("Unknown error: %s" % e)
        exit(1)

if not resp:
    print("Pod %s does not exits. Creating it..." % name)
    sys.exit(1)

resp = api.connect_get_namespaced_pod_exec(name, namespace,
                                           command=create_qmgmt_cmd('device list'),
                                           stderr=True, stdin=False,
                                           stdout=True, tty=False)
devices = {}
for res in [svc.split() for svc in resp.splitlines() if 'DATA' in svc]:
    if res[1] not in devices:
        devices[res[1]] = []
    devices[res[1]].append(res[0])

counter = 0
host3 = None
Host4 = None
for host, devices in devices.items():
    for device in devices:
        api.connect_get_namespaced_pod_exec(name, namespace,
                                            command=create_qmgmt_cmd(
                                                'device update add-tags {} host{}'.format(device, counter)),
                                            stderr=True, stdin=False,
                                            stdout=True, tty=False)

    counter += 1
    if counter == 3:
        host3 = host
    elif counter == 4:
        host4 = host

sys.exit(0)
