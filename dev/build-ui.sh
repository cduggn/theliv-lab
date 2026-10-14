#!/bin/sh
set -eu
# Angular 12's supported Node 14 runtime runs only inside this build container.
npm install --global npm@8.19.4 --no-audit --no-fund
if [ -f package-lock.json ]; then
  npm ci --legacy-peer-deps --no-audit --no-fund
else
  npm install --legacy-peer-deps --no-audit --no-fund
fi
./node_modules/.bin/ng build --configuration development --base-href /theliv/ --deploy-url /theliv/
