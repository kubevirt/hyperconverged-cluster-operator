from graphviz import Digraph

import configuration

KUBERNETES_IO_COMPONENT = "app.kubernetes.io/component"
KUBERNETES_IO_MANAGED_BY = "app.kubernetes.io/managed-by"


def get_labels(i):
    if "metadata" in i and "labels" in i["metadata"]:
        return i["metadata"]["labels"]
    return []


class GraphGenerator(object):

    def __init__(self, conf: configuration.Configuration, objects, outdir):
        self.conf = conf
        self.objects = objects
        self.outdir = outdir

    def generate(self):
        self.generate_component_graph()
        self.generate_managed_by_graph()

    def generate_managed_by_graph(self):
        dot = Digraph(comment='Managed By', graph_attr={'nodesep': '.5', 'ranksep': '5'})

        for i in self.objects:
            node_name = self.get_node_name(i)
            dot.node(node_name, node_name)

            labels = get_labels(i)
            for label in labels:
                if label == KUBERNETES_IO_MANAGED_BY:
                    managed_node_name = self.transform_alias(labels[KUBERNETES_IO_MANAGED_BY])
                    dot.node(managed_node_name, managed_node_name)
                    dot.edge(managed_node_name, node_name)

        self.render_graph(dot, "managed-by.gv")

    def generate_component_graph(self):
        # See https://stackoverflow.com/questions/7777722/top-down-subgraphs-left-right-inside-subgraphs
        dot = Digraph(comment='Component', graph_attr={'rankdir': 'TB'}, edge_attr={'style': 'invis', 'fontsize': '12'})

        component_dict = dict()
        for i in self.objects:
            node_name = self.get_node_name(i)
            dot.node(node_name, node_name)

            labels = get_labels(i)
            if KUBERNETES_IO_COMPONENT in labels:
                component_name = self.transform_component_alias(labels[KUBERNETES_IO_COMPONENT])

                if component_name not in component_dict:
                    component_dict[component_name] = []
                component_dict[component_name].append(node_name)

        for s in component_dict:
            prev = None
            with dot.subgraph(name='cluster_' + s, graph_attr={'nodesep': '3'}) as c:
                c.attr(label=s)
                for key in component_dict[s]:
                    c.node(key, key)
                    if prev is not None:
                        c.edge(prev, key)
                    prev = key

        self.render_graph(dot, "component.gv")

    def render_graph(self, g, name):
        g.render(directory=self.outdir, filename=name)

    def get_node_name(self, i):
        node_name = i["kind"] + "/" + i["metadata"]["name"]
        return self.transform_alias(node_name)

    def transform_alias(self, node_name):
        for key in self.conf.alias:
            if key == node_name:
                return self.conf.alias[key]
        return node_name

    def transform_component_alias(self, component_name):
        for key in self.conf.component_alias:
            if key == component_name:
                return self.conf.component_alias[key]
        return component_name
