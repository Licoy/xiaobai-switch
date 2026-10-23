// Installation smoke stays inside a fresh directory and never opens application state.
import fs from 'node:fs'
import path from 'node:path'
import os from 'node:os'
import {spawnSync} from 'node:child_process'
import {archiveTool,run} from './native-release-package.mjs'
import {assetName,targets,version} from './native-release-lib.mjs'
const target=process.argv[2],v=version(process.env.RELEASE_VERSION)
if(!targets.includes(target))throw new Error('invalid target')
const current=`${os.platform()}-${os.arch()}`
// Windows ARM64 is cross-built on x64; its install/runtime smoke requires a real ARM64 runner.
if(target!==current){console.log(`runtime smoke unavailable: built ${target}, host ${current}`);process.exit(0)}
const root=fs.mkdtempSync(path.join(os.tmpdir(),'xiaobai-native-install-')),archive=path.resolve('.build/native-release/assets',assetName(v,target))
try{
 const isolated={...process.env,HOME:root,XDG_DATA_HOME:path.join(root,'.local/share'),XDG_CONFIG_HOME:path.join(root,'.config'),XDG_CACHE_HOME:path.join(root,'.cache'),XIAOBAI_DEV_HOME:'invalid-relative-home'}
 let binary
 if(target.startsWith('linux')){
  const script=path.resolve('.build/native-release/assets/install.sh')
  const result=spawnSync('sh',[script,archive],{stdio:'inherit',env:isolated});if(result.status!==0)throw new Error('Linux installation smoke failed')
  binary=path.join(root,'.local/xiaobaiswitch.app/xiaobaiswitch')
 }else{
  run(archiveTool(isolated),['-xzf',archive,'-C',root],isolated)
  binary=target.startsWith('darwin')?path.join(root,'XiaoBaiSwitch.app/Contents/MacOS/XiaoBaiSwitch'):path.join(root,'XiaoBaiSwitch.exe')
 }
 const result=spawnSync(binary,['--native-smoke'],{encoding:'utf8',env:isolated})
 if(result.status!==0)throw new Error('installed package smoke failed')
 const metadata=JSON.parse(result.stdout)
 if(metadata.version!==v||metadata.os!==os.platform()||metadata.arch!==os.arch()||metadata.native!==true)throw new Error('package version/platform mismatch')
 console.log(JSON.stringify(metadata))
}finally{fs.rmSync(root,{recursive:true,force:true})}
