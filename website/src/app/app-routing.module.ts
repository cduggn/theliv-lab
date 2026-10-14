/* Copyright FMR LLC <opensource@fidelity.com>
 * SPDX-License-Identifier: Apache
 */
import { NgModule } from '@angular/core';
import { RouterModule, Routes } from '@angular/router';
import { KubePlatformComponent } from './kube-platform/kube-platform.component';

const routes: Routes = [
  { path: '', redirectTo: 'kubernetes/overview', pathMatch: 'full' },
  { path: 'home', redirectTo: 'kubernetes/overview', pathMatch: 'full' },
  { path: 'kubernetes', redirectTo: 'kubernetes/overview', pathMatch: 'full' },
  { path: 'kubernetes/overview', component: KubePlatformComponent },
  // Keep existing shared issue URLs usable; the workspace reloads their scope.
  { path: 'kubernetes/issue/:id', component: KubePlatformComponent }
];
@NgModule({
  imports: [RouterModule.forRoot(routes, { useHash: true })],
  exports: [RouterModule]
})
export class AppRoutingModule { }
