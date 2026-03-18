FROM grafana/k6:0.49.0

COPY traffic.js /scripts/traffic.js

ENTRYPOINT ["k6", "run", "/scripts/traffic.js"]
