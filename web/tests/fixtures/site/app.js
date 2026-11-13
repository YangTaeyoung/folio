import {next} from './counter.js';
let count = 0;
document.querySelector('#status').textContent = 'Relative ES modules ready';
document.querySelector('#counter').onclick = () => {
  count = next(count);
  document.querySelector('#counter').textContent = `Count: ${count}`;
};

const worker = new Worker(new URL('./worker.js', import.meta.url), {type:'module'});
worker.onmessage = e => { document.querySelector('#worker-status').textContent = e.data; worker.terminate(); };
if ('serviceWorker' in navigator) {
  navigator.serviceWorker.register('./service-worker.js').then(registration => {
    registration.unregister();
    document.querySelector('#service-worker-status').textContent = 'Unexpected registration';
  }).catch(() => { document.querySelector('#service-worker-status').textContent = 'Service worker rejected'; });
} else document.querySelector('#service-worker-status').textContent = 'Service worker rejected';
