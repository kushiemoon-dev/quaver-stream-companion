// Prints every event of the companion. Run with: node watch.mjs
const decoder = new TextDecoder();

async function watch(base) {
  const res = await fetch(base + '/events');
  if (!res.ok) throw new Error('HTTP ' + res.status);
  let buffer = '';
  for await (const chunk of res.body) {
    buffer += decoder.decode(chunk, { stream: true });
    let end;
    while ((end = buffer.indexOf('\n\n')) >= 0) {
      const block = buffer.slice(0, end);
      buffer = buffer.slice(end + 2);
      const data = block.split('\n').filter((l) => l.startsWith('data: ')).map((l) => l.slice(6)).join('\n');
      if (data) console.log(JSON.parse(data).type, data);
    }
  }
}

for (;;) {
  try {
    await watch('http://127.0.0.1:7878');
  } catch (err) {
    console.error('Disconnected:', err.message);
  }
  await new Promise((resolve) => setTimeout(resolve, 2000));
}
