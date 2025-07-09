import { Component, OnInit, Input } from '@angular/core';
import { FormGroup, FormBuilder, Validators } from '@angular/forms';
import { State } from '../State';

@Component({
  selector: 'app-query',
  templateUrl: './query.component.html',
  styleUrls: ['./query.component.css']
})
export class QueryComponent implements OnInit {

  @Input() state: State;

  queryForm: FormGroup;

  constructor(
    private formBuilder: FormBuilder, ) {
    this.queryForm = this.formBuilder.group({
      msisdn: ["905068866542", Validators.required]
    })
  }

  ngOnInit() {
  }

  query() {
    if (this.queryForm.valid) {
      this.state.msisdn = "" + this.queryForm.value.msisdn;
    }
  }

}
