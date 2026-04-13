const nunjucks = require('nunjucks');
const fs = require('fs-extra');
const path = require('path');
const chokidar = require('chokidar');
const browserSync = require('browser-sync').create();

const PAGES_DIR = path.join(__dirname, 'src/templates/pages');
const OUTPUT_DIR = __dirname;
const TEMPLATES_DIR = path.join(__dirname, 'src/templates');
const ASSETS_SRC = path.join(__dirname, 'src/assets');
const ASSETS_DEST = path.join(__dirname, 'assets');

nunjucks.configure(TEMPLATES_DIR, { 
    autoescape: true,
    noCache: true // 重要：开发模式下禁用缓存
});

async function build() {
    try {
        const files = await fs.readdir(PAGES_DIR);
        for (const file of files) {
            if (file.endsWith('.njk')) {
                const pageName = file.replace('.njk', '.html');
                const rendered = nunjucks.render(`pages/${file}`, {
                    site_name: 'Dawn Demo'
                });
                await fs.writeFile(path.join(OUTPUT_DIR, pageName), rendered);
                console.log(`Built ${pageName}`);
            }
        }

        // Copy assets
        if (await fs.pathExists(ASSETS_SRC)) {
            await fs.copy(ASSETS_SRC, ASSETS_DEST);
            console.log('Assets copied');
        }
    } catch (err) {
        console.error('Build failed:', err);
    }
}

function serve() {
    browserSync.init({
        server: OUTPUT_DIR,
        files: [path.join(OUTPUT_DIR, '*.html'), path.join(ASSETS_DEST, '**/*')],
        notify: false,
        open: false // 自动打开浏览器在某些环境下可能会有问题，手动打开即可
    });

    // 监听模板变化
    chokidar.watch(TEMPLATES_DIR).on('change', async (filePath) => {
        console.log(`Template changed: ${filePath}`);
        await build();
        browserSync.reload();
    });

    // 监听静态资源变化
    chokidar.watch(ASSETS_SRC).on('change', async (filePath) => {
        console.log(`Asset changed: ${filePath}`);
        await build();
        browserSync.reload();
    });
}

// 根据命令行参数决定是只构建还是进入开发模式
const isDev = process.argv.includes('--dev');

if (isDev) {
    build().then(() => serve());
} else {
    build();
}
