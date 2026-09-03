!function(){"use strict"
self.CACHE_BUSTER="1784189534924|0.15701697164886697",self.addEventListener("install",function(e){return self.skipWaiting()}),self.addEventListener("message",function(e){if("skipWaiting"===e.data)return self.skipWaiting()}),self.addEventListener("activate",function(e){return self.clients.claim()})
var e=["/v1/sys/storage/raft/snapshot"].map(function(e){let t=function(e,t=self.location){return decodeURI(new URL(encodeURI(e),t).toString())}(e)
return new RegExp(`^${t}$`)})
self.addEventListener("fetch",function(t){const n=t.request
return function(e,t){return!!t.find(t=>t.test(decodeURI(e)))}(n.url,e)&&"GET"===n.method?t.respondWith(function(e){const t=new Headers(e.headers)
return(n={action:"getToken"},self.clients.matchAll({includeUncontrolled:!0,type:"window"}).then(function(e){var t=e[0]
return new Promise(function(e,r){var s=new MessageChannel
s.port2.onmessage=function(t){t.data.error?r(t.data.error):e(t.data.token)},t.postMessage(n,[s.port1])})})).then(function(n){return t.set("X-Vault-Token",n),fetch(new Request(e.url,{method:e.method,headers:t}))})
var n}(n)):t.respondWith(fetch(n))})}()
