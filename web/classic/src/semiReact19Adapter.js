import { createRoot } from 'react-dom/client';
import semiGlobalEs from '@douyinfe/semi-ui/lib/es/_utils/semi-global';
import semiGlobalCjs from '@douyinfe/semi-ui/lib/cjs/_utils/semi-global';

const semiGlobals = [semiGlobalEs, semiGlobalCjs?.default || semiGlobalCjs];

semiGlobals.forEach((semiGlobal) => {
  if (semiGlobal?.config) {
    semiGlobal.config.createRoot = createRoot;
  }
});
