import argparse
import json
import subprocess
import sys

from eks import AwsCluster
from shell import run_out, run_in
from utils import reduce_subnets, get_current_region

DEFAULT_CIDR_BLOCK = "192.168.0.0/16"


def create_cluster(data):
    run_in("eksctl", "create", "cluster", "-f", "-", **data)


def generate_cluster_cfg(
        name,
        region,
        cidr,
        vpc_id,
        private,
        public,
        public_key_path,
):
    return {
        'apiVersion': 'eksctl.io/v1alpha5',
        'kind': 'ClusterConfig',

        'metadata': {
            'name': name,
            'region': region
        },
        'vpc': {
            'cidr': cidr,
            'id': vpc_id,
            'subnets': {
                'private': reduce_subnets(private),
                'public': reduce_subnets(public)
            }
        } if private and public and vpc_id else {
            'cidr': cidr,
            'nat': {'gateway': 'Single'},
            'clusterEndpoints': {'publicAccess': True, 'privateAccess': True}
        },
        'nodeGroups': [
            {
                'name': 'member-ng',
                'minSize': 2,
                'maxSize': 2,
                'instancesDistribution': {
                    'maxPrice': 0.093,
                    'instanceTypes': ["t3a.large", "t3.large"],
                    'onDemandBaseCapacity': 0,
                    'onDemandPercentageAboveBaseCapacity': 50,
                    'spotInstancePools': 2
                },
                'ssh': {
                    'publicKeyPath': public_key_path
                } if public_key_path else None,
                'iam': {
                    'withAddonPolicies': {
                        'externalDNS': True
                    }
                }
            }
        ]
    }


def authorize_security_group_ingress(**kwargs):
    sg_id = kwargs.get('sg_id')
    protocol = kwargs.get('protocol')
    port_range = kwargs.get('port_range')
    cidr = kwargs.get('cidr')
    region = kwargs.get('region')

    subprocess.call([
        "aws", "ec2", "authorize-security-group-ingress",
        "--group-id", str(sg_id),
        "--protocol", str(protocol),
        "--port", str(port_range),
        "--cidr", str(cidr),
        "--region", str(region),
    ])


def open_security_groups(cluster_name, region, private_subnets_cidrs, public_subnets_cidrs):
    res = run_out(
        "aws", "ec2", "describe-security-groups",
        "--region", region, "--filters",
        "Name=tag:aws:cloudformation:logical-id,Values=SG",
        "Name=tag:alpha.eksctl.io/cluster-name,Values=" + cluster_name
    )

    sg = res['SecurityGroups']
    if len(sg) < 1:
        raise Exception(
            "no security group found for cluster {0} nodegroup"
            .format(cluster_name)
        )

    sg_id = sg[0]['GroupId']

    # TODO: Open only the required ports
    # for now port 3389 was skipped
    if public_subnets_cidrs:
        for cidr in public_subnets_cidrs:
            for protocol in ['tcp', 'udp']:
                authorize_security_group_ingress(
                    sg_id=sg_id,
                    protocol=protocol,
                    port_range="1025-3388",
                    cidr=cidr,
                    region=region,
                )
                authorize_security_group_ingress(
                    sg_id=sg_id,
                    protocol=protocol,
                    port_range="3390-65535",
                    cidr=cidr,
                    region=region,
                )

    if private_subnets_cidrs:
        for cidr in private_subnets_cidrs:
            # opening all the ports for private subnets
            authorize_security_group_ingress(
                sg_id=sg_id,
                protocol="-1",
                port_range="-1",
                cidr=cidr,
                region=region,
            )


def main():
    parser = argparse.ArgumentParser(
        description='Utility for dealing with AWS clusters'
    )

    parser.add_argument(
        '--name',
        required=True,
        help='Member cluster name to create config for.'
    )
    parser.add_argument(
        '--region',
        required=False,
        help='Member cluster region'
    )
    parser.add_argument(
        '--ref',
        required=False,
        default=False,
        help='Reference cluster name (client cluster will use reference clusters vpc when is created)'  # noqa: E501
    )
    parser.add_argument(
        '--cidr',
        required=False,
        help='Client cluster name to create config yaml for.'
    )
    parser.add_argument(
        '--test',
        required=False,
        help='Dump generated config',
        action='store_true',
    )
    parser.add_argument(
        '--open-sg',
        required=False,
        help='Open all ports and all ips for SecurityGroups',
        dest='open_sg',
        action='store_true',
    )
    parser.add_argument(
        '--public-key-path',
        required=False,
        default=False,
        help='Your public ssh key. If provided it authorizes ssh connections and adds the specified ssh key.',  # noqa: E501
        dest='public_key_path',
    )

    args = parser.parse_args()

    cidr = args.cidr if args.cidr else DEFAULT_CIDR_BLOCK
    region = args.region if args.region else get_current_region()

    priv_subnets, pub_subnets, vpc_id = None, None, None
    if args.ref:
        reference_cluster = AwsCluster(args.ref, region)
        priv_subnets = reference_cluster.get_subnets("Private")
        pub_subnets = reference_cluster.get_subnets("Public")
        vpc_id = reference_cluster.get_vpc_id()

    cfg = generate_cluster_cfg(
        args.name,
        region,
        cidr,
        vpc_id,
        priv_subnets,
        pub_subnets,
        args.public_key_path,
    )
    if args.test:
        json.dump(cfg, sys.stdout, indent=4)
        return

    create_cluster(cfg)

    if not args.ref:
        reference_cluster = AwsCluster(args.name, region)
        priv_subnets = reference_cluster.get_subnets("Private")
        pub_subnets = reference_cluster.get_subnets("Public")

    if args.open_sg:
        priv_subnets, pub_subnets = priv_subnets or [], pub_subnets or []

        # getting subnets cidrs
        private_subnets_cidrs = [
            subnet['CidrBlock'] for subnet in priv_subnets
        ]
        public_subnets_cidrs = [
            subnet['CidrBlock'] for subnet in pub_subnets
        ]

        open_security_groups(
            args.name,
            region,
            private_subnets_cidrs,
            public_subnets_cidrs,
        )


if __name__ == '__main__':
    main()
