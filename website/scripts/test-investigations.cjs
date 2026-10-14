// Exercise adapters and asynchronous workspace state without requiring a browser.
// Angular's build separately checks the templates and component bindings.
const path = require('path');
process.chdir(path.resolve(__dirname, '..'));
require('ts-node').register({ project: 'tsconfig.spec.json', compilerOptions: { module: 'commonjs' } });
const Jasmine = require('jasmine');
const runner = new Jasmine();
runner.loadConfig({
  spec_dir: '.',
  spec_files: [
    'src/app/kube-platform/investigation.spec.ts',
    'src/app/kube-platform/kube-platform.component.spec.ts'
  ],
  random: false
});
runner.execute();
