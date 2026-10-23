// Run only on an ephemeral Windows CI runner, using an isolated registry namespace and temp files.
import fs from 'node:fs'
import assert from 'node:assert/strict'
import os from 'node:os'
import path from 'node:path'
import {spawnSync} from 'node:child_process'
if(process.platform!=='win32')throw new Error('installer lifecycle fixtures require Windows')
if(process.env.CI!=='true')throw new Error('installer fixtures require an ephemeral CI runner')
const root=fs.mkdtempSync(path.join(os.tmpdir(),'xiaobai-installer-fixture-')),payload=path.join(root,'payload'),install=path.join(root,'installed'),marker=path.join(root,'args.json'),key=`Software\\XiaoBaiSwitchInstallerFixture-${process.pid}`
function run(cmd,args,expected=0){const r=spawnSync(cmd,args,{encoding:'utf8',env:{...process.env,NATIVE_INSTALL_SMOKE_OUT:marker}});if(r.status!==expected)throw new Error(`${path.basename(cmd)}: ${r.status}\n${r.stdout}\n${r.stderr}`);return r}
try{
 fs.mkdirSync(payload);fs.mkdirSync(install)
 const source=path.join(root,'main.go')
 fs.writeFileSync(source,`package main\nimport("encoding/json";"fmt";"os")\nfunc main(){b,_:=json.Marshal(os.Args[1:]);if p:=os.Getenv("NATIVE_INSTALL_SMOKE_OUT");p!=""{if e:=os.WriteFile(p,b,0600);e!=nil{panic(e)}};fmt.Println("{\\\"version\\\":\\\"1.0.0\\\",\\\"native\\\":true}")}\n`)
 run(process.env.NATIVE_REAL_GO??'go',['build','-o',path.join(payload,'XiaoBaiSwitch.exe'),source])
 fs.writeFileSync(path.join(install,'old-marker'),'old payload')
 const exe=path.join(root,'installer.exe'),compiler=process.env.MAKENSIS??'makensis'
 const params=['/DFIXTURE',`/DSHORTCUT=${path.join(root,'shortcut.lnk')}`,`/DVERSION=1.0.0`,`/DFILEVERSION=1.0.0.0`,`/DPAYLOAD=${payload}`,`/DOUTPUT=${exe}`,`/DUNINSTALL=${key}\\Uninstall`,`/DMANU=${key}\\Manufacturer`,`/DPROTOCOL=xiaobai-installer-fixture-${process.pid}`]
 run(compiler,[...params,'scripts/native-transition-installer.nsi'])
 // Node quotes argv for Windows; literal quote characters would reach the restarted app.
 // Tauri updater supplies /UPDATE /ARGS; /D must be the final NSIS switch.
 const expectedArgs=['--native-smoke','argument with spaces']
 run(exe,['/S','/R','/UPDATE','/ARGS',...expectedArgs,`/D=${install}`])
 const deadline=Date.now()+5000
 let args=[],lastReadError
 while(Date.now()<deadline){
  if(fs.existsSync(marker)){try{args=JSON.parse(fs.readFileSync(marker,'utf8'))}catch(error){lastReadError=error;args=[]}}
  if(Array.isArray(args)&&args.length===expectedArgs.length&&args.every((arg,index)=>arg===expectedArgs[index]))break
  Atomics.wait(new Int32Array(new SharedArrayBuffer(4)),0,0,10)
 }
 const shape={count:Array.isArray(args)?args.length:null,nativeSmoke:Array.isArray(args)&&args.includes(expectedArgs[0]),spacedArgument:Array.isArray(args)&&args.includes(expectedArgs[1]),quotedSpacedArgument:Array.isArray(args)&&args.includes('\"argument with spaces\"'),parseFailed:!!lastReadError}
 assert.deepEqual(Array.isArray(args)?args.map((arg,index)=>arg===expectedArgs[index]):null,expectedArgs.map(()=>true),`updater restart arguments changed: ${JSON.stringify(shape)}`)
 if(fs.existsSync(path.join(install,'old-marker')))throw new Error('upgrade did not replace payload')
 // Copy failure must preserve the previously installed program and report failure.
 fs.writeFileSync(path.join(install,'survives-rollback'),'before')
 run(compiler,[...params,'/DFAIL_AFTER_COPY','scripts/native-transition-installer.nsi'])
 const failed=spawnSync(exe,['/S','/UPDATE',`/D=${install}`],{encoding:'utf8'})
 if(failed.status===0)throw new Error('injected copy failure returned success')
 if(!fs.existsSync(path.join(install,'survives-rollback'))||!fs.existsSync(path.join(install,'XiaoBaiSwitch.exe')))throw new Error('failed upgrade lost previous application')
 console.log('NSIS /UPDATE /ARGS restart and copy-failure recovery passed')
}finally{
 spawnSync('reg',['delete',`HKCU\\${key}`,'/f'],{stdio:'ignore'})
 spawnSync('reg',['delete',`HKCU\\Software\\Classes\\xiaobai-installer-fixture-${process.pid}`,'/f'],{stdio:'ignore'})
 fs.rmSync(root,{recursive:true,force:true})
}
