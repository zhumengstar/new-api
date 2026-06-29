import { registerBrowserEnv } from '@visactor/vchart';

if (typeof window !== 'undefined') {
  registerBrowserEnv();
}
