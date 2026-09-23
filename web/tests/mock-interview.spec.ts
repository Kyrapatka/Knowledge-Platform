import { test, expect } from "@playwright/test";

test("mock: independent multi-folder plans, distributions, all depths, replacement, mobile and validation", async ({page}) => {
  test.setTimeout(240000);
  const errors:string[]=[];page.on("pageerror",e=>errors.push(e.message));
  await page.goto("/");
  await page.getByRole("button",{name:"Create an account",exact:true}).click();
  await page.getByLabel("Username",{exact:true}).fill(`mock${Date.now().toString(36)}`);
  await page.getByLabel("Password",{exact:true}).fill("TestKnowledge2026!");
  const auth=page.waitForResponse(r=>r.url().endsWith("/auth/browser/register"));
  await page.getByRole("button",{name:"Create account",exact:true}).click();
  const headers={Authorization:`Bearer ${(await (await auth).json()).access_token}`};
  await expect(page.getByRole("heading",{name:"My library."})).toBeVisible();
  await page.getByRole("link",{name:"Mock interview",exact:true}).click();
  await page.getByRole("button",{name:"Import question bank",exact:true}).click();
  const modal=page.getByRole("dialog");
  await expect(modal.locator(".interview-seed-domains label")).toHaveCount(15);
  for(const label of await modal.locator(".interview-seed-domains label").all()) {
    const name=(await label.locator("span").innerText()).split("\n")[0];
    await label.getByRole("checkbox").setChecked(["Go","Databases","Networking"].includes(name));
  }
  const imported=page.waitForResponse(r=>r.url().endsWith("/interview/seed/import")&&r.request().method()==="POST");
  await modal.getByRole("button",{name:"Import selected subjects"}).click();
  expect((await imported).ok()).toBeTruthy();await expect(modal).not.toBeVisible();
  const library=await (await page.request.get("/api/v1/library",{headers})).json();
  expect(library.folders).toHaveLength(3);
  for(const folder of library.folders) {
    const response=await page.request.post("/api/v1/training/plans",{headers,data:{source_folder_ids:[folder.id],algorithm_key:"interview_long_term",horizon_days:150,pool_size:5}});
    expect(response.status()).toBe(201);
  }
  for(const title of ["English one","English two"]) expect((await page.request.post("/api/v1/folders",{headers,data:{title,template_key:"english_words"}})).status()).toBe(201);
  const plansBefore=await (await page.request.get("/api/v1/training/plans",{headers})).json();
  await page.reload();
  await expect(page.locator(".interview-source")).toHaveCount(3);
  await page.getByRole("button",{name:"Select all",exact:true}).click();
  await expect(page.locator(".interview-sources input:checked")).toHaveCount(3);
  await expect(page.getByText("All selected (3)",{exact:true})).toBeVisible();
  await page.getByRole("button",{name:"Clear all",exact:true}).click();
  await expect(page.getByText("0 selected",{exact:true})).toBeVisible();
  await expect(page.getByRole("button",{name:"Start interview",exact:true})).toBeDisabled();
  await page.locator(".interview-source").filter({hasText:"Interview / Go"}).getByRole("checkbox").check();
  await page.locator(".interview-source").filter({hasText:"Interview / Networking"}).getByRole("checkbox").check();
  await page.getByRole("checkbox",{name:/Bank testing mode/}).check();
  await expect(page.locator(".interview-distribution")).toContainText("78.9%");
  await expect(page.locator(".interview-distribution")).toContainText("21.1%");
  await page.getByRole("button",{name:"Select all",exact:true}).click();
  for(const mode of ["real","balanced","custom","deep1","deep2","deep3"]) {
    if(mode!=="real") {await page.getByRole("button",{name:"Select all",exact:true}).click();await page.getByRole("checkbox",{name:/Bank testing mode/}).check();}
    await page.getByRole("combobox",{name:"Interview mode",exact:true}).selectOption(mode.startsWith("deep")?"deep":mode);
    if(mode.startsWith("deep")) await page.getByRole("combobox",{name:"Depth level",exact:true}).selectOption(mode.slice(-1));
    if(mode==="balanced") await expect(page.locator(".interview-distribution")).toContainText("33.3%");
    if(mode==="custom") {
      for(const label of ["Go","SQL","HTTP / Networks"]) await page.getByRole("spinbutton",{name:`${label} weight`,exact:true}).fill("0");
      await expect(page.getByText(/choose at least one available topic with a positive weight/i)).toBeVisible();
      await expect(page.getByRole("button",{name:"Start interview",exact:true})).toBeDisabled();
      for(const [label,value] of [["Go","5"],["SQL","3"],["HTTP / Networks","2"]]) await page.getByRole("spinbutton",{name:`${label} weight`,exact:true}).fill(value);
      await expect(page.locator(".interview-distribution")).toContainText("50.0%");
      for(const width of [320,360,390,430,768,1440]) {
        await page.setViewportSize({width,height:900});
        expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1)).toBeTruthy();
        await page.locator(".interview-weights").screenshot({path:`test-results/mock-custom-${width}.png`});
      }
    }
    const started=page.waitForResponse(r=>r.url().endsWith("/training/mock-interviews")&&r.request().method()==="POST");
    await page.getByRole("button",{name:"Start interview",exact:true}).click();
    const result=await started;expect(result.status()).toBe(200);let view=await result.json();
    expect(view.graph.state.practice_only).toBe(true);expect(view.current.interview_graph.review_credit).toBe(false);
    expect(view.graph.state.interview_plan.strategy.target_roots).toBe(mode==="deep1"?4:mode==="deep2"?3:mode==="deep3"?2:6);
    await expect(page.locator(".interview-question")).toBeVisible();
    if(mode==="real") {
      await page.getByRole("button",{name:"Dismiss bank testing warning"}).click();
      await expect(page.locator(".interview-bank-warning")).toHaveCount(0);
      const roots=new Set<string>();
      for(let i=0;i<20;i++) {
        const response=page.waitForResponse(r=>r.url().endsWith("/actions"));
        await page.getByRole("button",{name:"Next Root",exact:true}).click();
        const received=await response;expect(received.status()).toBe(200);view=(await received.json()).session;
        expect(view.session.status).toBe("active");expect(view.graph.state.current_root).toBe(1);expect(view.graph.state.answered_questions).toBe(0);expect(view.graph.state.completed_root_ids||[]).toHaveLength(0);roots.add(view.current.material_id);
      }
      expect(roots.size).toBeGreaterThan(10);
      await expect(page.getByTestId("completed-roots")).toHaveText("0");
      await expect(page.locator(".interview-bank-warning")).toHaveCount(0);
      const question=await page.locator(".interview-question").innerText();await page.reload();await expect(page.locator(".interview-question")).toHaveText(question);
    }
    await page.getByRole("button",{name:"End interview",exact:true}).click();
    await expect(page.getByRole("heading",{name:"A little more prepared."})).toBeVisible();
    await page.getByRole("button",{name:"Choose another interview"}).click();
  }
  expect(await (await page.request.get("/api/v1/training/plans",{headers})).json()).toEqual(plansBefore);
  await page.goto(`/folders/${library.folders[0].id}`);
  await page.getByRole("button",{name:"Edit folder",exact:true}).click();
  for(const width of [320,360,390,430,768]) {
    await page.setViewportSize({width,height:900});
    const dialog=page.getByRole("dialog",{name:"Edit folder"});
    for(const label of ["Save changes","Cancel","Delete folder"]) {const button=dialog.getByRole("button",{name:label,exact:true});await button.scrollIntoViewIfNeeded();await expect(button).toBeInViewport();}
    expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1)).toBeTruthy();
  }
  expect(errors).toEqual([]);
});
