const puppeteer = require('puppeteer');

async function setupGitea(username, password, email, gitName, repoName) {
  const browser = await puppeteer.launch({ headless: false }); // Set to false for debugging
  const page = await browser.newPage();

  try {
    await page.goto('http://localhost:3000/', { waitUntil: 'networkidle0' });

    // Initial Configuration
    await page.waitForSelector('button[type="submit"]');

    // Fill out the initial configuration form
    await page.select('#db_type', 'PostgreSQL');
    await page.type('#db_host', 'db:5432');
    await page.type('#db_user', 'gitea');
    await page.type('#db_passwd', 'gitea_password');
    await page.type('#db_name', 'gitea');
    await page.type('#app_name', 'Gitea: Git with a cup of tea');
    await page.type('#repo_root_path', '/data/git/repositories');
    await page.type('#run_user', 'git');
    await page.type('#domain', 'localhost');
    await page.type('#ssh_port', '22');
    await page.type('#http_port', '3000');
    await page.type('#app_url', 'http://localhost:3000/');
    await page.type('#log_root_path', '/data/gitea/log');

    // Click "Install Gitea" button
    await page.click('button[type="submit"]');

    // Wait for installation to complete
    await page.waitForNavigation({ waitUntil: 'networkidle0' });

    console.log('Initial configuration completed');

    // User Registration
    await page.goto('http://localhost:3000/user/sign_up', { waitUntil: 'networkidle0' });

    // Fill out the registration form
    await page.type('#user_name', username);
    await page.type('#email', email);
    await page.type('#password', password);
    await page.type('#retype', password);

    // Submit the registration form
    await page.click('button[type="submit"]');

    // Wait for registration to complete
    await page.waitForNavigation({ waitUntil: 'networkidle0' });

    console.log('User registration successful');
  } catch (error) {
    console.error('Error during Gitea setup:', error);
    throw error;
  } finally {
    await browser.close();
  }
}

setupGitea(process.argv[2], process.argv[3], process.argv[4], process.argv[5], process.argv[6]);
