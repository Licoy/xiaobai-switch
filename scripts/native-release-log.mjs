// Persist only redacted output, including failures, without logging command arguments.
import fs from 'node:fs'
import path from 'node:path'
import {fileURLToPath} from 'node:url'
import {spawn} from 'node:child_process'
export function redact(text,env) {
 const values=[...new Set(Object.entries(env).filter(([name,value])=>value&&/(?:KEY|PASSWORD|CERT|TOKEN|APPLE_ID)/.test(name)).flatMap(([,value])=>[value,...value.split(/\r?\n/).filter(Boolean)]))].sort((a,b)=>b.length-a.length)
 for(const value of values)text=text.replaceAll(value,value.replace(/[^\r\n]+/g,'[REDACTED]'))
 return text
}
export function logCommand(file,command,args,env=process.env) {
 fs.mkdirSync(path.dirname(file),{recursive:true})
 const log=fs.createWriteStream(file,{flags:'a'})
 const child=spawn(command,args,{env,stdio:['ignore','pipe','pipe']})
 for(const [stream,output] of [[child.stdout,process.stdout],[child.stderr,process.stderr]]) {
  stream.setEncoding('utf8')
  let pending=''
  const write=text=>{const clean=redact(text,env);output.write(clean);log.write(clean)}
  stream.on('data',chunk=>{pending+=chunk;const end=pending.lastIndexOf('\n');if(end>=0){write(pending.slice(0,end+1));pending=pending.slice(end+1)}})
  stream.on('end',()=>{if(pending)write(pending)})
 }
 child.on('error',error=>{const message=redact(error.message,env)+'\n';process.stderr.write(message);log.write(message)})
 child.on('close',code=>{log.end();process.exitCode=code??1})
}
if(process.argv[1]&&path.resolve(process.argv[1])===fileURLToPath(import.meta.url))logCommand(process.argv[2],process.argv[3],process.argv.slice(4))
