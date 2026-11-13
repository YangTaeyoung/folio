import {next} from './counter.js';
let count = 0;
document.querySelector('#status').textContent = 'Relative ES modules ready';
document.querySelector('#counter').onclick = () => {
  count = next(count);
  document.querySelector('#counter').textContent = `Count: ${count}`;
};
