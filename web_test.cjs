const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

// Minimal DOM surface; the real app.js renders the response into these nodes.
class Element {
  constructor() {this.children = []; this.textContent = ''; this.classList = {toggle() {}};}
  addEventListener() {}
  append(...nodes) {this.children.push(...nodes);}
  replaceChildren(...nodes) {this.children = nodes;}
}

test('a delayed pre-save status cannot replace newer status and history', async () => {
  const nodes = new Map();
  const pending = [];
  const context = vm.createContext({
    document: {
      getElementById(id) {if (!nodes.has(id)) nodes.set(id, new Element()); return nodes.get(id);},
      createElement() {return new Element();}
    },
    AbortSignal, Date, setTimeout() {},
    fetch(url) {
      // Leave initial configuration loading pending; exercise status requests directly.
      if (url === '/api/config') return new Promise(() => {});
      assert.equal(url, '/api/status');
      return new Promise(resolve => pending.push(resolve));
    }
  });
  vm.runInContext(fs.readFileSync(path.join(__dirname, 'web/app.js'), 'utf8'), context);
  const beforeSave = context.refreshPanel();
  const afterSave = context.refreshPanel();
  function status(historyID, current, message) {
    return {running:false, phase:'stopped', current, failures:0, pool_size:200, results:[],
      history_id:historyID, event_count:1,
      events:[{id:1, time:new Date().toISOString(), level:'info', message}]};
  }
  pending[1]({ok:true, json:async () => status('new-history', 'new-country', 'new-event')});
  await afterSave;
  pending[0]({ok:true, json:async () => status('old-history', 'old-country', 'old-event')});
  await beforeSave;
  assert.equal(nodes.get('current-group').textContent, 'new-country');
  const renderedMessage = nodes.get('event-list').children[0].children[2].textContent;
  assert.equal(renderedMessage, 'new-event');
});

test('recovery controls serialize configuration and display progress', () => {
  const nodes = new Map();
  const get = id => {if (!nodes.has(id)) nodes.set(id, new Element()); return nodes.get(id);};
  const context = vm.createContext({document:{getElementById:get, createElement:() => new Element()},
    AbortSignal, Date, setTimeout() {}, fetch:() => new Promise(() => {})});
  vm.runInContext(fs.readFileSync(path.join(__dirname,'web/app.js'),'utf8'),context);
  for (const [id,value] of Object.entries({'api-url':'http://mihomo:9090',secret:'','target-group':'🚀 节点选择',interval:'5',threshold:'1',timeout:'4','recovery-interval':'60','recovery-threshold':'3'})) get(id).value=value;
  get('recovery-enabled').checked=true;
  get('recovery-initial-delay').value='300';
  const payload=context.payload();
  assert.equal(payload.recovery_initial_delay_seconds,300);
  assert.equal(payload.recovery_enabled,true);
  assert.equal(payload.recovery_interval_seconds,60);
  assert.equal(payload.recovery_success_threshold,3);
  assert.equal('recovery_cooldown_seconds' in payload,false);
  context.renderStatus({phase:'healthy',running:true,results:[],recovery:{enabled:true,candidate:'香港',successes:2,threshold:3,message:'等待下一轮恢复检查'}});
  assert.match(get('recovery-status').textContent,/香港.*2\/3/);
  assert.doesNotMatch(get('recovery-status').textContent,/冷却/);
  context.renderStatus({phase:'healthy',running:true,results:[],recovery:{enabled:true,initial_wait_seconds:250}});
  assert.match(get('recovery-status').textContent,/首次检查.*250/);
  context.renderStatus({phase:'stopped',results:[],recovery:{enabled:false}});
  assert.equal(get('recovery-status').textContent,'自动恢复未运行');
});
