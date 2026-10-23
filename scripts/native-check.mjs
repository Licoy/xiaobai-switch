import {spawnSync} from 'node:child_process'
import fs from 'node:fs'
import path from 'node:path'
import {targets,version} from './native-release-lib.mjs'
const buildVersion=version(JSON.parse(fs.readFileSync('mygo.json','utf8')).version)
const go=process.env.NATIVE_REAL_GO || path.resolve('.build/toolchain/go/bin/go')
const env={...process.env,CGO_ENABLED:'0',GOTOOLCHAIN:'local'}
const out=path.resolve('.build/native-check');fs.mkdirSync(out,{recursive:true})
function run(args,extra={}){const r=spawnSync(go,args,{stdio:'inherit',env:{...env,...extra}});if(r.status!==0)process.exit(r.status??1)}
const tool=spawnSync(go,['version'],{encoding:'utf8',env});if(tool.status!==0||!tool.stdout.includes('go1.27.1 '))throw new Error('Go 1.27.1 is required')
const module=spawnSync(go,['list','-m','-f','{{.Version}}','github.com/egoist/mygo'],{encoding:'utf8',env});if(module.status!==0||module.stdout.trim()!=='v0.6.2')throw new Error('mygo v0.6.2 is required')
console.log(tool.stdout.trim())
for(const target of targets){const [GOOS,GOARCH]=target.split('-');const bin=path.join(out,'XiaoBaiSwitch-'+target+(GOOS==='windows'?'.exe':''));run(['build','-trimpath','-ldflags',`-X github.com/licoy/xiaobai-switch/internal/buildinfo.Version=${buildVersion}`,'-o',bin,'.'],{GOOS,GOARCH});if(!fs.statSync(bin).size)throw new Error('empty binary');console.log(`built ${target}: ${bin}`)}
