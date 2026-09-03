define("@embroider/macros/es-compat2",["exports"],function(e){"use strict"
Object.defineProperty(e,"__esModule",{value:!0}),e.default=function(e){return e?.__esModule?e:{default:e,...e}}}),define("@embroider/macros/runtime",["exports"],function(e){"use strict"
function o(e){return n.packages[e]}function r(){return n.global}Object.defineProperty(e,"__esModule",{value:!0}),e.config=o,e.each=function(e){if(!Array.isArray(e))throw new Error("the argument to the each() macro must be an array")
return e},e.getGlobalConfig=r,e.isTesting=function(){let e=n.global,o=e&&e["@embroider/macros"]
return Boolean(o&&o.isTesting)},e.macroCondition=function(e){return e},e.setTesting=function(e){n.global||(n.global={})
n.global["@embroider/macros"]||(n.global["@embroider/macros"]={})
n.global["@embroider/macros"].isTesting=Boolean(e)}
const n=globalThis.__embroider_macros__runtime_config__||={}
n.packages||={},n.global||={}
const t={packages:{},global:{}}
Object.assign(n.packages,t.packages),Object.assign(n.global,t.global)
let a="undefined"!=typeof window?window._embroider_macros_runtime_config:void 0
if(a){let e={config:o,getGlobalConfig:r,setConfig(e,o){n.packages[e]=o},setGlobalConfig(e,o){n.global[e]=o}}
for(let o of a)o(e)}})
