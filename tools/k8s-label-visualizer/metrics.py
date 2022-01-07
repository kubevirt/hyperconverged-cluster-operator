import json
import os
import subprocess

import configuration

MEMORY_QUERY='sum by (label_app_kubernetes_io_component) (sum(container_memory_usage_bytes{namespace="openshift-cnv"}) by (pod) * on (pod) group_left(label_app_kubernetes_io_component) kube_pod_labels{namespace="openshift-cnv"}) / (1024* 1024)'
CPU_QUERY='sum by (label_app_kubernetes_io_component) (sum(pod:container_cpu_usage:sum{namespace="openshift-cnv"}) by (pod) * on (pod) group_left(label_app_kubernetes_io_component) kube_pod_labels{namespace="openshift-cnv"})'

METRIC_LABEL_FOR_COMPONENT = "label_app_kubernetes_io_component"


class GraphGenerator(object):

    def __init__(self, conf: configuration.Configuration, outdir):
        self.conf = conf
        self.outdir = outdir

    def generate(self):
        output_file = os.path.join(self.outdir, "metrics.txt")
        file = open(output_file, "w")
        print_to_file(file, "MEMORY CONSUMPTION", self.run_prometheus_query(MEMORY_QUERY))
        print_to_file(file, "CPU CONSUMPTION", self.run_prometheus_query(CPU_QUERY))
        file.close()

    def run_prometheus_query(self, prometheus_query):
        oc_command = "oc exec -n openshift-monitoring prometheus-k8s-0 -c prometheus -- " \
                     "curl --silent --data-urlencode \'query={}\' " \
                     "http://127.0.0.1:9090/api/v1/query".format(prometheus_query)
        result_as_json_string = subprocess.check_output(oc_command, shell=False, stderr=subprocess.STDOUT)
        result = json.loads(result_as_json_string.decode('UTF-8'))

        if result['status'] != 'success':
            raise Exception("Result for prometheus query is not success. Result: {}".format(result))

        return self.convert_to_dic_per_component(result['data']['result'])

    def convert_to_dic_per_component(self, raw_dict):
        """
        Iterate over raw prometheus query result and convert it to a simple dictionary
        :param raw_dict: .data.result part of prometheus query result
        :return: a dictionary with component names as keys and query results as values
        """
        result = {}
        for item in raw_dict:
            if METRIC_LABEL_FOR_COMPONENT in item['metric']:
                key = item['metric'][METRIC_LABEL_FOR_COMPONENT]
            else:
                key = "unassigned"

            key = self.transform_component_alias(key)
            result[key] = item['value'][1]

        return result

    def transform_component_alias(self, component_name):
        for key in self.conf.component_alias:
            if key == component_name:
                return self.conf.component_alias[key]
        return component_name


def print_to_file(file, title, data):
    file.write(title)
    file.write("\n")
    for item in data:
        file.write("{} : {}\n".format(item, data[item]))

    file.write("\n\n\n")
