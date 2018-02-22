#!/bin/bash -e

#for profile in profiles/*mb.yml; do
for profile in $(ls -Sr profiles/*mb.yml);do
    echo $profile
    docker restart localtesting_6.3.0-SNAPSHOT_apm-server
    sleep 60

    NOW=$(date '+%Y-%m-%d %H:%M:%S')
    ./loadbeat -e -E loadbeat.base_urls=["http://localhost:8200/"] -E 'output.elasticsearch.hosts=["localhost:9200"]' -E loadbeat.run_timeout=5m -E loadbeat.request_timeout=1m -c $profile &> ${profile}.log
    echo ${profile} ${NOW} - $(date '+%Y-%m-%d %H:%M:%S')
done

