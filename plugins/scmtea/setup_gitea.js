import puppeteer from 'puppeteer';

async function setupGitea(username, email, password) {
  const browser = await puppeteer.launch({ headless: false });
  const page = await browser.newPage();
  const timeout = 30000; // 30 seconds timeout
  page.setDefaultTimeout(timeout);

  try {
    await page.setViewport({ width: 1975, height: 1302 });

    console.log('Navigating to Gitea installation page...');
    await page.goto('http://localhost:3000/', { waitUntil: 'networkidle0' });

    console.log('Opening Administrator Account Settings...');
    await page.waitForSelector('xpath=/html/body/div[1]/div/div/div/div/form/div[15]/details[3]/summary');
    await page.click('xpath=/html/body/div[1]/div/div/div/div/form/div[15]/details[3]/summary');

    console.log('Filling in Administrator Username...');
    await page.waitForSelector('xpath=/html/body/div[1]/div/div/div/div/form/div[15]/details[3]/div[1]/input');
    await page.type('xpath=/html/body/div[1]/div/div/div/div/form/div[15]/details[3]/div[1]/input', username);

    console.log('Filling in Email Address...');
    await page.waitForSelector('xpath=/html/body/div[1]/div/div/div/div/form/div[15]/details[3]/div[2]/input');
    await page.type('xpath=/html/body/div[1]/div/div/div/div/form/div[15]/details[3]/div[2]/input', email);

    console.log('Filling in Password...');
    await page.waitForSelector('xpath=/html/body/div[1]/div/div/div/div/form/div[15]/details[3]/div[3]/input');
    await page.type('xpath=/html/body/div[1]/div/div/div/div/form/div[15]/details[3]/div[3]/input', password);

    console.log('Confirming Password...');
    await page.waitForSelector('xpath=/html/body/div[1]/div/div/div/div/form/div[15]/details[3]/div[4]/input');
    await page.type('xpath=/html/body/div[1]/div/div/div/div/form/div[15]/details[3]/div[4]/input', password);

    console.log('Clicking Install Gitea button...');
    await page.waitForSelector('xpath=/html/body/div[1]/div/div/div/div/form/div[18]/div[2]/button');
    await page.click('xpath=/html/body/div[1]/div/div/div/div/form/div[18]/div[2]/button');

    console.log('Waiting for navigation after installation...');
    await page.waitForNavigation({ waitUntil: 'networkidle0', timeout: 60000 });

    console.log('Gitea installation completed successfully');
    return { success: true, message: 'Gitea installation completed successfully' };
  } catch (error) {
    console.error('Error during Gitea setup:', error);
    await page.screenshot({ path: 'error-screenshot.png', fullPage: true });
    return { success: false, message: `Gitea setup failed: ${error.message}. Screenshot saved as error-screenshot.png` };
  } finally {
    await browser.close();
  }
}

// Main execution
const [, , username, email, password] = process.argv;
setupGitea(username, email, password)
  .then((result) => {
    console.log(JSON.stringify(result));
  })
  .catch(error => {
    console.error('Setup failed:', JSON.stringify({ success: false, message: error.message }));
    process.exit(1);
  });

export { setupGitea };
