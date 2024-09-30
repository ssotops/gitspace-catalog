const puppeteer = require('puppeteer');

async function setupGitea(username, password, email, gitName, repoName) {
  const browser = await puppeteer.launch({ headless: false });
  const page = await browser.newPage();
  const timeout = 60000;
  page.setDefaultTimeout(timeout);

  try {
    console.log('Starting Gitea setup...');

    // Initial Configuration
    await page.setViewport({ width: 1280, height: 800 });
    await page.goto('http://localhost:3000/', { waitUntil: 'networkidle0' });
    console.log('Navigated to Gitea homepage');

    // Check if we're on the installation page
    const installButton = await page.$('button.ui.primary.button');
    if (installButton) {
      console.log('On installation page. Proceeding with initial configuration.');

      // Click the "Install Gitea" button
      console.log('Clicking "Install Gitea" button...');
      await installButton.click();
      await page.waitForNavigation({ waitUntil: 'networkidle0' });
      console.log('Initial configuration completed.');
    } else {
      console.log('Installation page not found. Assuming Gitea is already installed.');
    }

    // User Registration
    console.log('Navigating to registration page...');
    await page.goto('http://localhost:3000/user/sign_up', { waitUntil: 'networkidle0' });

    // Check if we're on the registration page
    const registrationForm = await page.$('form.ui.form');
    if (registrationForm) {
      console.log('On registration page. Filling out the form...');

      // Wait for the form fields to be available
      await page.waitForSelector('#user_name', { visible: true, timeout });
      await page.waitForSelector('#email', { visible: true, timeout });
      await page.waitForSelector('#password', { visible: true, timeout });
      await page.waitForSelector('#retype', { visible: true, timeout });

      await page.type('#user_name', username);
      await page.type('#email', email);
      await page.type('#password', password);
      await page.type('#retype', password);

      console.log('Submitting registration form...');
      await Promise.all([
        page.click('button.ui.primary.button'),
        page.waitForNavigation({ waitUntil: 'networkidle0' })
      ]);

      const currentUrl = page.url();
      if (currentUrl.includes('/user/login')) {
        console.log('Registration successful. Redirected to login page.');
      } else {
        console.log('Registration may have failed or led to an unexpected page:', currentUrl);
      }
    } else {
      console.log('Registration form not found. User might already exist or there might be an issue.');
      console.log('Current URL:', page.url());
      console.log('Page content:', await page.content());
    }

    // Login (whether registration succeeded or user already exists)
    console.log('Attempting to log in...');
    await page.goto('http://localhost:3000/user/login', { waitUntil: 'networkidle0' });
    await page.type('#user_name', username);
    await page.type('#password', password);
    await page.click('button.ui.primary.button');
    await page.waitForNavigation({ waitUntil: 'networkidle0' });

    // Check if login was successful
    const dashboardElement = await page.$('.dashboard');
    if (dashboardElement) {
      console.log('Logged in successfully.');
    } else {
      console.log('Login may have failed. Current URL:', page.url());
      throw new Error('Login failed');
    }

    // Create repository
    console.log('Creating repository...');
    await page.goto('http://localhost:3000/repo/create', { waitUntil: 'networkidle0' });
    await page.type('#repo_name', repoName);
    await page.click('button.ui.primary.button');
    await page.waitForNavigation({ waitUntil: 'networkidle0' });

    // Final check
    const repoUrl = `http://localhost:3000/${username}/${repoName}`;
    await page.goto(repoUrl, { waitUntil: 'networkidle0' });
    if (page.url() === repoUrl) {
      console.log('Gitea setup completed successfully!');
    } else {
      console.log('There might be an issue with the repository creation.');
      console.log('Current URL:', page.url());
    }

  } catch (error) {
    console.error('Error during Gitea setup:', error);
    await page.screenshot({ path: 'error-screenshot.png', fullPage: true });
    throw error;
  } finally {
    await browser.close();
  }
}

setupGitea(process.argv[2], process.argv[3], process.argv[4], process.argv[5], process.argv[6]);
